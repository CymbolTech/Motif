# Measurement

Measurement never awards anything; rules do that.

- **Goal:** a target count of one event type, such as 100 completed orders.
- **Campaign:** one event type inside an optional UTC window, optionally linked to a goal. Campaign analytics report event count, unique users and the goal target.
- **Metric:** a named `count` or `unique_users` definition over one event type.
- **Funnel:** 2 to 8 ordered event steps; users advance at most once per step, in order.
- **Retention:** a cohort event and a return event at 1 to 5 day intervals (1 to 90 days).
- **Segment:** users with at least `N` occurrences of one event in an optional window.
- **Experiment report:** 2 to 4 weighted variants and a conversion event. Assignment is deterministic per user; the product serves the variant itself.

Analytics read the events table directly and suit the current deployment's scale, not a data warehouse. Reports are available in the console under Reports.
