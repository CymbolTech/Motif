# Rule language

A rule is `when <event_type>, if <conditions>, then <actions>`, plus an optional schedule window. Conditions are data, not code.

## Conditions

Each condition object has exactly one operator. Paths use dot notation from the payload root: `$.total`, `$.order.category`.

| Operator | Example | Meaning |
| --- | --- | --- |
| `{}` | `{}` | Always matches (the default) |
| `all` | `{"all":[{"eq":["$.payment_status","paid"]},{"gte":["$.total",100]}]}` | Every child matches |
| `any` | `{"any":[{"eq":["$.plan","pro"]},{"eq":["$.plan","team"]}]}` | At least one child matches |
| `exists` | `{"exists":"$.coupon"}` | The path is present |
| `eq`, `neq` | `{"eq":["$.status","completed"]}` | Equal, not equal |
| `gt`, `gte`, `lt`, `lte` | `{"gte":["$.score",80]}` | Numeric or string comparison |

- A missing path makes every comparison false, except `neq`, which is true.
- Comparing values of different types (the string `"100"` with the number `100`) is an error, and the worker cannot process that event. `check_rule` flags it; fix the payload type or the literal.
- Arrays and objects are not valid comparison operands.
- Limits: 8 levels of nesting, 64 expressions, 32 children per `all` or `any`.
- `all` and `any` stop at the first deciding child, so later children are not evaluated.

## Actions

Send `actions` as an ordered list:

| Action | Fields | Effect |
| --- | --- | --- |
| `award_xp` | positive integer `amount` | Adds non-spendable XP to the event's user |
| `award_points` | `currency` code, positive integer `amount` | Appends an award to that currency's ledger |
| `log` | optional `message` | Records the match without changing user state |

Award actions need the event's `user_id`. The currency must exist and be active in the environment.

## Versions and publishing

Creating a rule saves version 1 as a draft. Editing saves a new draft version. Only the published version runs; publishing a new version archives the previous one. Drafts never execute.

## Schedule windows

`starts_at` is inclusive and `ends_at` exclusive, both UTC. The worker checks the window when it processes the event.

## Several rules on one event

Every published rule whose conditions match runs, independently. Give rules names that make the combined effect obvious, and use separate rules for separate outcomes (books get XP, home goods get coins).
