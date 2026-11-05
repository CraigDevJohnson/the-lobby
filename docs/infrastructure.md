# How the site runs

Working sketch, drawn during the baseline discussion. Not a decision record.

## 1. Where the reactivity lives

"Static" describes how the files are delivered, not how the page behaves. The React app is a set of files that are the same for everyone (HTML, JavaScript, CSS). Once the browser has them, the JavaScript runs **in the browser**: ticking a Team, switching between Visitor and Member views, filtering games, all happen there instantly, with no server involved. The server is only called when the page needs data (search Teams, fetch a schedule, who am I) or something only the server may do (sign-in, build a calendar, write to Google).

```mermaid
flowchart LR
    subgraph browser["Person's browser"]
        app["React app<br/>(JavaScript running locally)<br/>renders what is shown<br/>from current state + role"]
    end
    subgraph edge["Cloudflare"]
        cache["Cache of app files"]
        access["Access<br/>(sign-in paths and /admin)"]
    end
    subgraph aws["AWS, Oregon"]
        apigw["API Gateway"]
        site["Site Lambda (Go/Chi)<br/>serves app files + landing HTML<br/>sign-in session, who-am-I<br/>API for every Tool<br/>link addresses"]
    end
    app -- "app files, once" --> cache
    app -- "data: JSON requests" --> access --> apigw --> site
    cache -. "miss" .-> apigw
```

Two consequences:

- The app files can be cached by Cloudflare, so the Site Lambda rarely serves them.
- The browser is never trusted. It *shows* Member sections because the server told it the role, but every Member action is checked again on the server using the Sign-in session cookie.

## 2. The whole picture

```mermaid
flowchart TB
    person["Person's browser<br/>(React app)"]
    calapps["Calendar apps<br/>Apple · Google · Outlook<br/>fetch Session and Member links"]

    subgraph cf["Cloudflare"]
        dns["DNS + TLS + cache"]
        acc["Access: proves identity on sign-in paths and /admin<br/>emailed code or Google"]
        bots["Bot Fight Mode off<br/>Browser Integrity Check off on link paths"]
    end

    subgraph aws["AWS, Oregon (the account Craig uses for his other workloads)"]
        apigw["API Gateway (HTTP)"]
        site["Site Lambda (Go/Chi)<br/>repo: the-lobby<br/>React files · landing HTML<br/>sign-in sessions · who-am-I<br/>calls Tool backends"]
        siteDb[("DynamoDB<br/>sign-in sessions, Members, grants,<br/>Remembered teams, Member links")]
        soccer["Let's Plan Soccer backend Lambda (Go/Chi)<br/>repo: soccer (assumed name)<br/>teams · schedules · calendars<br/>Connected calendars<br/>daily update"]
        soccerDb[("DynamoDB<br/>Google access (encrypted, assumed),<br/>events the site added")]
        sched["EventBridge Scheduler<br/>once a day"]
        mc["Minecraft backend Lambda<br/>(later, own repo)"]
    end

    lps["League's schedule service<br/>(undocumented)"]
    google["Google Calendar API"]

    person --> dns --> acc --> apigw --> site
    calapps --> dns
    site --> siteDb
    site -- "signed request<br/>(Member named in a header)" --> soccer
    site -. "signed request" .-> mc
    soccer --> soccerDb
    soccer --> lps
    soccer --> google
    sched --> soccer
```

Every arrow into AWS is a short request that starts a Lambda and ends when the response is sent. The only work that is not a request is the daily update, which the scheduler starts. If the Access trial fails, Cognito takes Access's place; nothing else in the picture moves.

## 3. The soccer page, step by step

```mermaid
sequenceDiagram
    participant B as Browser (React app)
    participant CF as Cloudflare
    participant S as Site Lambda
    participant SB as Let's Plan Soccer backend
    participant L as League service

    B->>CF: GET /soccer
    CF-->>B: app files (from cache after the first time)
    Note over B: App starts, asks who it is talking to
    B->>S: GET /api/me (cookie if any)
    S-->>B: visitor, or member + Tools granted
    Note over B: App shows the Visitor view,<br/>or the Member view with Remembered teams
    B->>S: GET /api/soccer/teams?q=rovers
    S->>SB: signed request
    SB->>L: search
    L-->>SB: teams
    SB-->>S: JSON
    S-->>B: JSON
    Note over B: Person ticks Teams, unticks games:<br/>rendered locally, no requests
    B->>S: POST /api/soccer/calendar (chosen games)
    S->>SB: signed request
    SB-->>S: .ics or Session link
    S-->>B: download / link
```

A Member's actions (add a Remembered team, replace their Member link) go to the Site Lambda alone, which refuses them without a Sign-in session holding the `soccer` grant and keeps Remembered teams and the link in its own table ([ADR 0011](adr/0011-member-soccer-calendar-in-the-site-table.md)). Adding a Team reads it through the backend first. When a calendar app fetches a Member link, the Site Lambda finds the Member from the link's secret and sends their Remembered teams to the backend, which answers the calendar and any Team it moved on to its next Session; the backend never learns who the Member is.

## 4. Lambda or a container

| | Lambda (chosen) | ECS Fargate container |
|---|---|---|
| Runs | per request, then stops | 24 hours a day |
| Cost at this traffic | about $0 (free tier covers it) | about $9 a month for the smallest task, plus about $16 for a load balancer or a few dollars for a public address |
| Idle | nothing to keep alive | process always up |
| Cold start | a few hundred ms for Go after a quiet spell | none |
| Fits | short requests, scheduled jobs | long-lived connections, long-running processes |

Nothing in the site needs a process that stays up. Highly interactive pages are not the deciding factor: the interaction happens in the browser either way. A container would start to make sense only for something like streaming a live Minecraft console over an open connection, and even then API Gateway's WebSocket mode is the serverless answer. The Minecraft Server itself runs on its own Machine and is outside this picture.
