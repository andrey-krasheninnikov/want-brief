# Domain glossary

Use these terms in documentation and issue titles. Their meanings follow the current domain model.

| Term | Meaning |
| --- | --- |
| Item | A unit of work or information kept in the task collection. It has a kind, status, priority flags, and optional project, dates, people, and time intervals. |
| Item kind | The item's category: task, request, note, agreement, obligation, initiative, or life. |
| Item status | The item's workflow position, such as backlog, in progress, blocked, review, done, or cancelled. Some statuses apply only to tasks. |
| Source | A Jira, Todoist, or manual origin for items. Connected sources keep sync status and settings. |
| Source sync schedule | A per-source interval between sync attempts. A source can remain in manual mode or sync every 15 minutes, hour, six hours, or day. |
| Application backup | A JSON copy of application records, relationships, and settings. Source tokens, account credentials, and sessions are excluded. Restoring replaces current application records. |
| Project | A named grouping for work, with links, people, companies, and target hours. |
| Person | A contact record that can be related to work, projects, events, and other people. |
| Company | An organization record with related people and career information. |
| Event | A scheduled event or call, once or recurring. |
| Event occurrence | One dated instance of an event series. An occurrence may be overridden or skipped. |
| Event time zone | The time zone saved on an event series that anchors its recurring local time. |
| Quadrant | An Eisenhower priority group: do, schedule, delegate, or drop. |
| Occupancy | How an item uses schedule capacity: solo, parallel, or waiting. |
| Schedule | A calculated placement of work around events and capacity, including risk and overflow. |
| Schedule settings | The time zone, work window, capacity, and packing preferences used to calculate a schedule. |
| What-if comparison | A schedule calculation for a proposed change that does not save that change. |
| Check-in | A recorded stress, focus, energy, interest, or happiness level. |
| Time interval | A start and optional end time recorded against an item. Several intervals can overlap. |
| Allocated time | The sum of interval durations. Overlapping intervals each contribute their full duration. |
| Wall-clock time | Elapsed time covered by intervals, with overlap counted once. |
| Earn day | Earnings recorded for one date in USD and RUB, with an exchange rate. |
