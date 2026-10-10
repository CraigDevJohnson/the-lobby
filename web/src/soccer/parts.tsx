// Small pieces the Schedule Downloader's sections share.

// The league's own site, for when its schedule service is not answering.
const LEAGUE_SITE = "https://www.letsplaysoccer.com/";

export function Dots() {
  return (
    <span className="lobby-status-dots" aria-hidden="true">
      <span />
      <span />
      <span />
    </span>
  );
}

// The league's schedule service is down: say so plainly, offer a retry and
// the league's own site. Nothing is worked around or served from memory.
export function LeagueDown({ onRetry, what = "Teams and games can’t be looked up until it’s back." }: { onRetry?: () => void; what?: string }) {
  return (
    <div className="soccer-notice" role="alert">
      <p className="soccer-notice-title">The league’s schedule service isn’t answering.</p>
      <p>
        {what} You can check <a href={LEAGUE_SITE}>Let’s Play Soccer</a> in the meantime.
      </p>
      {onRetry && (
        <button type="button" className="quiet-button" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}

export function ToolDown({ onRetry, what = "Nothing you’ve chosen is lost." }: { onRetry?: () => void; what?: string }) {
  return (
    <div className="soccer-notice" role="alert">
      <p className="soccer-notice-title">The Schedule Downloader isn’t answering right now.</p>
      <p>{what} Please try again in a moment.</p>
      {onRetry && (
        <button type="button" className="quiet-button" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}

type IconName = "close" | "plus" | "chevron" | "link" | "arrow" | "download";

const PATHS: Record<IconName, string> = {
  close: "M6 6l12 12M18 6 6 18",
  plus: "M12 5v14M5 12h14",
  chevron: "M6 9.5l6 6 6-6",
  link: "M10 14a4.5 4.5 0 0 0 6.4 0l3-3a4.5 4.5 0 0 0-6.4-6.4l-1.2 1.2M14 10a4.5 4.5 0 0 0-6.4 0l-3 3a4.5 4.5 0 0 0 6.4 6.4l1.2-1.2",
  arrow: "M4 12h15M13 5.5 19.5 12 13 18.5",
  download: "M12 4v11M7 10.5l5 5 5-5M5 20h14",
};

// Line icons in the Tool glyphs' stroke family; always decorative.
export function Icon({ name }: { name: IconName }) {
  return (
    <svg className="soccer-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d={PATHS[name]} />
    </svg>
  );
}
