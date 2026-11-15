import { useCallback, useEffect, useState } from "react";
import { TOOLS } from "../tools";
import { backendWords, calendarWords, getSite, siteLead, versionWords, type Site } from "./site";

// The Site card on Lobby Admin (#98): whether each Tool's backend answered a
// check just now, whether the calendars are offered, and the version
// running, so the Owner can tell whether trouble is the site or a Tool. It
// shares the Members card's shell and rows.

const nameOf = (id: string) => TOOLS.find((t) => t.id === id)?.name ?? id;

const STILL_UNCHECKED = "Checked just now. We still couldn’t check the site.";

export function SiteCard() {
  // undefined while the first check runs; null when the site couldn't say.
  const [site, setSite] = useState<Site | null | undefined>(undefined);
  const [checking, setChecking] = useState(true);
  const [said, setSaid] = useState("");

  useEffect(() => {
    let live = true;
    void getSite().then((got) => {
      if (!live) return;
      setSite(got);
      setChecking(false);
    });
    return () => {
      live = false;
    };
  }, []);

  // Check again keeps the last answer on screen while it asks, then says
  // that it checked, so the same answer twice still reads as new.
  const again = useCallback(async () => {
    if (checking) return;
    const before = site;
    setChecking(true);
    setSaid("");
    const got = await getSite();
    setSite(got);
    setChecking(false);
    setSaid(got ? "Checked just now." : before === null ? STILL_UNCHECKED : "");
  }, [checking, site]);

  return (
    <section className="mcx la-site" aria-labelledby="la-site-title">
      <h2 id="la-site-title" className="mcx-title">
        Site
      </h2>
      <p className="mcx-lead">{siteLead(window.location.host, site?.environment ?? "")}</p>
      {site === undefined && <p className="mcx-quiet">Checking the site…</p>}
      {site === null && (
        <p className="mcx-quiet" role="alert">
          We couldn’t check the site just now.
        </p>
      )}
      {site && (
        <dl className="mcx-list">
          {site.backends.map((b) => (
            <div key={b.tool} className="mcx-item">
              <dt className="mcx-name">{nameOf(b.tool)}</dt>
              <dd>{backendWords(b)}</dd>
            </div>
          ))}
          <div className="mcx-item">
            <dt className="mcx-name">Google and iCloud calendars</dt>
            <dd>{calendarWords(site.calendars)}</dd>
          </div>
          <div className="mcx-item">
            <dt className="mcx-name">Running version</dt>
            <dd>{versionWords(site.version)}</dd>
          </div>
        </dl>
      )}
      {site !== undefined && (
        <button
          className="quiet-button la-site-again"
          type="button"
          aria-disabled={checking || undefined}
          onClick={() => void again()}
        >
          {checking ? "Checking…" : "Check again"}
        </button>
      )}
      <p className="mcx-said" role="status">
        {said}
      </p>
    </section>
  );
}
