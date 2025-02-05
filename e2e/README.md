# End to End testing

## Verify manual run

1. Run main server: `GIN_MODE=release /usr/local/go/bin/go run cmd/cmd.go server --config config.toml 2&>1 >outfile`
2. (Optional) Run async worker: `go run cmd/cmd.go -queue`
3. Create Subscription:

   ```sh
   cat payload.json | http POST :8081/subscriptions
   ```

4. Send event:

   ```sh
   echo '{
        "owner_id": "owner123",
        "event_type": "webhook.abc",
        "payload": {"a": "abc", "b": "def"}
   }' | http post :8082/event
   ```

## Load test

1. cd into `e2e/k6` directory
2. Run load test: `BASE_URL=http://localhost ./run.sh`
