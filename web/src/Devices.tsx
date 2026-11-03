import { useEffect, useState } from "react";
import { Robot } from "./Robot";

// The family devices page at /admin (ADR 0010): the two configuration
// profiles and how to install each. It sits behind its own Cloudflare Access
// application for the adults and never looks at the Sign-in session. The two
// links are in the prerendered HTML, so the page reads without JavaScript;
// after load it asks the server whether the profiles are configured yet and
// says so when they are not. Nothing on the page is private: the files are,
// and those are only ever served behind Access.

type Profile = {
  device: "kids" | "adult";
  name: string;
  file: string;
  summary: string;
  steps: string[];
};

const PROFILES: Profile[] = [
  {
    device: "kids",
    name: "Kids iPad",
    file: "kids-ipad.mobileconfig",
    summary:
      "Sends the iPad’s internet through the family’s filter, keeps that filter from being switched off, and asks a parent before a new app is installed.",
    steps: [
      "Open this link in Safari on the iPad, then Settings › General › VPN & Device Management, choose the downloaded profile, Install.",
      "The iPad must already be supervised. Once the profile is on, later changes to it go through Apple Configurator on the Mac.",
    ],
  },
  {
    device: "adult",
    name: "Adult iPhone",
    file: "adult-iphone.mobileconfig",
    summary:
      "Trusts the family filter’s certificate, and keeps the phone on the family filter’s DNS while the Cloudflare One Agent is off.",
    steps: [
      "Open this link in Safari on the iPhone, then Settings › General › VPN & Device Management, choose the downloaded profile, Install.",
      "Then Settings › General › About › Certificate Trust Settings, and turn on full trust for the Cloudflare Gateway certificate.",
    ],
  },
];

type Readiness = { kind: "checking" } | { kind: "ready" } | { kind: "not-configured" } | { kind: "unknown" };

async function checkReadiness(): Promise<Readiness> {
  let res: Response;
  try {
    res = await fetch("/api/devices/profiles", {
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
  } catch {
    return { kind: "unknown" };
  }
  if (res.ok) return { kind: "ready" };
  let code: unknown;
  try {
    code = ((await res.json()) as { code?: unknown }).code;
  } catch {
    // Not one of the site's own answers.
  }
  return code === "not_configured" ? { kind: "not-configured" } : { kind: "unknown" };
}

const STATUS: Record<Exclude<Readiness["kind"], "checking">, string> = {
  ready: "Both profiles are ready to install.",
  "not-configured":
    "The profiles aren’t configured yet: the family filter hasn’t been set up, or its details haven’t reached the site. The links below answer with an error until then.",
  unknown: "Couldn’t check whether the profiles are ready. The links below still work if they are.",
};

export function FamilyDevices() {
  const [readiness, setReadiness] = useState<Readiness>({ kind: "checking" });

  useEffect(() => {
    let cancelled = false;
    void checkReadiness().then((r) => {
      if (!cancelled) setReadiness(r);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const notConfigured = readiness.kind === "not-configured";

  return (
    <div className="page">
      <header className="page-header">
        <div className="page-header-inner">
          <a className="identity identity-compact" href="/">
            The Lobby
          </a>
          <a className="page-back" href="/">
            Back to The Lobby
          </a>
        </div>
      </header>

      <main className="devices-ground">
        <div className="devices">
          <div className="devices-opening">
            <h1 className="devices-title">Family devices</h1>
            <p className="devices-lead">The two profiles for the family’s devices, and how to put each one on.</p>
            <p className="devices-status" role="status">
              {readiness.kind === "checking" ? "" : STATUS[readiness.kind]}
            </p>
          </div>

          <div className="devices-list">
            {PROFILES.map((p) => (
              <section key={p.device} className="devices-profile" aria-labelledby={`profile-${p.device}`}>
                <div className="devices-profile-about">
                  <h2 id={`profile-${p.device}`} className="devices-profile-name">
                    {p.name}
                  </h2>
                  <p className="devices-profile-summary">{p.summary}</p>
                  <a
                    className="tactile tactile-small devices-download"
                    href={`/admin/profiles/${p.file}`}
                    aria-disabled={notConfigured || undefined}
                  >
                    Download {p.file}
                  </a>
                </div>
                <ol className="devices-steps">
                  {p.steps.map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>
              </section>
            ))}
          </div>
        </div>
      </main>

      <footer className="page-footer">
        <div className="page-footer-inner">
          <Robot className="lobby-robot" />
          <a href="/">Back to The Lobby</a>
        </div>
      </footer>
    </div>
  );
}
