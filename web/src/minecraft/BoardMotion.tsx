import { Component, useLayoutEffect, useRef, type HTMLAttributes, type ReactNode, type RefObject } from "react";
import { mayMove, RISE_AFTER_MS, RISE_MS, SETTLE_MS, settles, tracks } from "./motion";
import { EXPO_OUT, SPRING } from "./plots/stage";

// The board's own moves (DESIGN.md, "Board moves"; the rules are in
// motion.ts). The plots show what happened to a world; these carry the eye
// through what that did to the board around it. Each is a one-shot that
// answers a press or a status answer, a newer change cuts one still running,
// and with every move switched off the board is still right.

type Kept = { el: HTMLElement; rows: string; width: number };

// A settle under way: its move, and the rows it runs between.
type Flight = { move: Animation; from: string; to: string };

type SettleProps = HTMLAttributes<HTMLElement> & {
  as?: "div" | "p" | "section";
  // A plain box, which settles its height. Without it the element is a grid
  // and settles its rows.
  box?: boolean;
  // The grids inside this one that settle in its place wherever they have
  // rows of their own: each world, where the worlds stack. The worlds under
  // a changed one ride with it, as anything after a settling row does.
  within?: string;
  children?: ReactNode;
};

// Settle is an element whose rows glide to their new heights when what they
// hold changes, instead of jumping: the row opens over what has just arrived
// or closes over what has gone, and everything after it on the page follows.
// The words and controls themselves change at once, so the page never shows
// an older state than it knows.
//
// It is a class because only getSnapshotBeforeUpdate reads the page before
// React changes it. That gives the rows as they really were, part-way through
// an earlier settle included, so a second change picks up where the first had
// got to. It sees the changes its parent renders; one a child makes on its
// own simply cuts.
export class Settle extends Component<SettleProps, object, Kept[] | null> {
  private el: HTMLElement | null = null;
  private flights = new Map<HTMLElement, Flight>();
  // Notes what React changes under this element. The page re-renders for
  // many reasons (an announcement, a status answer that says the same); an
  // update that changed nothing in here leaves the rows, and any settle
  // under way, alone: what was read before it is dropped, and nothing is
  // lifted off or read again.
  private changes: MutationObserver | null = null;

  private keep = (el: HTMLElement | null) => {
    this.el = el;
  };

  // A window that changes width lays the rows out afresh, perhaps as the
  // other kind of grid (side by side or stacked), so the heights a settle
  // was gliding between are no longer the rows'. It is cut, and the page is
  // simply right. A change of height alone (a phone's toolbar coming or
  // going) is not a new layout.
  private wide = 0;
  private cut = () => {
    const wide = document.documentElement.clientWidth;
    if (wide === this.wide) return;
    this.wide = wide;
    for (const { move } of this.flights.values()) move.cancel();
  };

  componentDidMount() {
    this.wide = document.documentElement.clientWidth;
    window.addEventListener("resize", this.cut);
    if (!this.el || typeof MutationObserver !== "function") return;
    this.changes = new MutationObserver(() => {});
    this.changes.observe(this.el, { subtree: true, childList: true, characterData: true, attributes: true });
  }

  // untouched says whether this update left everything under the element as
  // it was. The observer's notes are read before they are delivered, so they
  // are this update's own; marking a settle is no change to what a row holds.
  private untouched(): boolean {
    const notes = this.changes?.takeRecords();
    return notes !== undefined && notes.every((note) => note.attributeName === "data-settling");
  }

  private read(el: HTMLElement): Kept {
    const box = el.getBoundingClientRect();
    return { el, rows: this.props.box ? `${box.height}px` : getComputedStyle(el).gridTemplateRows, width: box.width };
  }

  // fly glides an element's rows from one set of heights to another, from
  // the start or from part-way along.
  private fly(el: HTMLElement, from: string, to: string, at: CSSNumberish | null) {
    const move = el.animate(
      { [this.props.box ? "height" : "gridTemplateRows"]: [from, to] },
      { duration: SETTLE_MS, easing: EXPO_OUT },
    );
    if (at !== null) move.currentTime = at;
    this.flights.set(el, { move, from, to });
    // While it settles, the stylesheet holds each row to its track and cuts
    // it there, so what has just arrived is uncovered as the row opens.
    el.dataset.settling = "";
    const land = () => {
      if (this.flights.get(el)?.move !== move) return;
      this.flights.delete(el);
      delete el.dataset.settling;
    };
    move.finished.then(land, land);
  }

  // grids is whose rows settle: the grids inside this one where they have
  // rows of their own (the worlds, stacked), and otherwise this one (the
  // board, whose rows the worlds share side by side). Never both: a grid
  // whose rows are being moved would stretch the grids inside it.
  private grids(): HTMLElement[] {
    const el = this.el;
    if (!el) return [];
    if (this.props.box || !this.props.within) return [el];
    const inside = [...el.querySelectorAll<HTMLElement>(this.props.within)].filter(
      (grid) => tracks(getComputedStyle(grid).gridTemplateRows) !== null,
    );
    return inside.length ? inside : [el];
  }

  getSnapshotBeforeUpdate(): Kept[] | null {
    return this.el ? this.grids().map((grid) => this.read(grid)) : null;
  }

  componentDidUpdate(_props: SettleProps, _state: object, before: Kept[] | null) {
    if (!before || this.untouched()) return;
    // Everything in here moves or nothing does, so the grids stay in step: a
    // world scrolled out of view still settles while any of the board can be
    // seen.
    const wanted = this.el !== null && mayMove(this.el);
    // Rows can only be read at rest, so every settle under way is lifted off
    // before any grid is read, and none is started until all have been.
    const lifted = before.map((was) => {
      const flying = this.flights.get(was.el);
      const at = flying ? flying.move.currentTime : null;
      flying?.move.cancel();
      return { was, flying, at };
    });
    const read = lifted.map((grid) => ({ ...grid, now: grid.was.el.isConnected ? this.read(grid.was.el) : null }));
    for (const { was, flying, at, now } of read) {
      // A different width is the window changing, not the row's contents.
      if (!now || !wanted || Math.abs(now.width - was.width) > 0.5) continue;
      if (flying && tracks(now.rows) !== null && !settles(flying.to, now.rows)) {
        // These rows came out the same (something else in them changed):
        // the settle carries on from where it had got to. Rows that can no
        // longer be read are another kind of grid now, and are left alone.
        this.fly(was.el, flying.from, flying.to, at);
      } else if (settles(was.rows, now.rows)) {
        this.fly(was.el, was.rows, now.rows, null);
      }
    }
  }

  componentWillUnmount() {
    window.removeEventListener("resize", this.cut);
    this.changes?.disconnect();
    this.changes = null;
    for (const { move } of this.flights.values()) move.cancel();
  }

  render() {
    const { as, box: _box, within: _within, children, ...rest } = this.props;
    const Tag = (as ?? "div") as "div";
    return (
      <Tag ref={this.keep} {...rest}>
        {children}
      </Tag>
    );
  }
}

// useRise lifts a custard control onto its coral base when it becomes the
// thing to press while the page is open: Copy address as its world turns
// ready, Start when a world can be started again. It waits pressed flat on
// the card until its row has all but settled, then springs up, so the depth
// that means "press me" arrives as a move. Never on arrival, and a press or a
// newer state cuts it.
export function useRise(ref: RefObject<HTMLElement | null>, pressable: boolean) {
  const was = useRef(pressable);
  const rising = useRef<Animation | null>(null);
  useLayoutEffect(() => {
    const before = was.current;
    was.current = pressable;
    rising.current?.cancel();
    rising.current = null;
    const el = ref.current;
    if (before || !pressable || !el || !mayMove(el)) return;
    const css = getComputedStyle(el);
    const lift = css.getPropertyValue("--lift").trim();
    const coral = css.getPropertyValue("--coral").trim();
    if (!lift || !coral) return;
    const move = el.animate(
      [
        {
          offset: 0,
          transform: `translateY(${lift})`,
          boxShadow: `0 0 0 -1px ${coral}, 0 6px 14px -6px rgb(35 53 82 / 0)`,
        },
      ],
      { duration: RISE_MS, delay: RISE_AFTER_MS, easing: SPRING, fill: "backwards" },
    );
    rising.current = move;
    // A press is answered by the control's own sink, at once.
    const press = () => move.cancel();
    el.addEventListener("pointerdown", press);
    el.addEventListener("keydown", press);
    const done = () => {
      el.removeEventListener("pointerdown", press);
      el.removeEventListener("keydown", press);
      if (rising.current === move) rising.current = null;
    };
    move.finished.then(done, done);
  }, [pressable, ref]);
  useLayoutEffect(() => () => rising.current?.cancel(), []);
}
