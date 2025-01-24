# Metrics

## Prometheus metrics

We are using following Prometheus metrics:

| Short name                        | Type      | Description                                                                                           |
| --------------------------------- | --------- | ----------------------------------------------------------------------------------------------------- |
| myoprelay_ingest_total            | counter   | Total number of events ingested (but no yet dispatched or consumed)                                   |
| myoprelay_ingest_consumed_total   | counter   | Total number of events ingested and processed (with success or error)                                 |
| myoprelay_ingest_errors_total     | counter   | Total number of events ingested and resulted with errors (Non-200 client response)                    |
| myoprelay_ingest_success_total    | counter   | Total number of events ingested and resulted with success (200 client HTTP response)                  |
| myoprelay_ingest_latency          | Histogram | Time since between event start till event acknowledgement                                             |
| myoprelay_end_to_end_latency      | Histogram | Time difference between event start and event's target's response (includes client latency)           |
| myoprelay_event_dispatch_latency  | Histogram | Time difference between event start and event acknowledgement by worker                               |
| myoprelay_event_preflight_latency | Histogram | Time difference between event start and before event target hit (excludes client latency)             |
| myoprelay_total_subscriptions     | Gauge     | Total number of events subscriptions (different company. Excludes multiple subscriptions per company) |
| myoprelay_fanout_size             | Histogram | Total number of targets per event (bucketed, see notes below)                                         |

## Notes

- Fanout depends on how much we want to fanout per event. If total number of subscriptions per event exceeds a certain threshold, we may want to limit the
  fanout using token bucket algorithm
