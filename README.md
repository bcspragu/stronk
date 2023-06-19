# stronk

Stronk is a simple web app for tracking your exercise. It was built with [Jim Wendler's 5/3/1](https://www.amazon.com/Simplest-Effective-Training-System-Strength/dp/B00686OYGQ) in mind, but is decently flexible (at least in terms of routines, the UI is pretty heavily based around the main four 5/3/1 lifts).

Your exercises are configured via a `routine.json` file that specifies lifts in a rough hierarchy:

* Week - A week of workouts. A week can be 'optional'. This is used for deload weeks.
* Day - A day of workouts
* Movement - A set of lifts, all having the same exercise (e.g. squat, bench) and set type (e.g. warmup, assistance, etc)
* Set - A number of target reps at a target percentage of the training max for that movement's exercise. Can optionally be 'to failure', meaning the rep target is a minimum

An example `routine.example.json` is included, which implements a fairly standard 5/3/1 using "Big but Boring" for the assistance work. It includes an optional deload week.

## Screenshots

The training max page, where you enter your initial training maxes, which all subsequent sets will be based on.

![Screenshot of the training max page, showing four inputs corresponding to the four lifts of 5/3/1, along with a selector for the smallest plate you have available at your gym](/screenshots/training-maxes.png)

The lift/main page, where you get an overview of the day's lifts, and record them as you do them, adding rep counts and notes as relevant.

![Screenshot of the lifts page, showing a series of sets broken into warmup, main, and assistance. The bottom half of the page shows buttons for recording lifts, adding notes, skipping, and more](/screenshots/lifts.png)


## Local Development

To run locally, you'll need a recent version of Go + some version of NPM. Install frontend dependencies (namely Svelte) with `cd frontend && npm install`.

Then, to run the infrastructure.

```bash
# Run the backend
go run .

# In another terminal
cd frontend
npm run dev
```

The server stores lift info in a SQLite database, which will be created + migrated on the first boot.

Frontend is available at `localhost:5173`, backend is `localhost:8080`.

## Deployment

Note: the app has no authentication, make sure to introduce basic auth or deploy the app behind something like [Tailscale](https://tailscale.com/)

The main way to deploy this is with two Docker containers `fivethreeone` and `fivethreeone-fe`, which run the backend and frontend respectively. I run this in a local K8s deployment, using a config like:

<details>

<summary>fivethreeone.yaml</summary>

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fivethreeone-deployment
  labels:
    app: fivethreeone
spec:
  selector:
    matchLabels:
      app: fivethreeone
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: fivethreeone
    spec:
        containers:
        - image: <registry>/fivethreeone-fe
          name: frontend
          env:
          - name: PUBLIC_API_BASE_URL
            value: "http://localhost:8080"
          ports:
            - containerPort: 3000
              name: web
        - image: <registry>/fivethreeone
          name: backend
          env:
          - name: ROUTINE_FILE
            value: /config/routine.json
          - name: DB_FILE
            value: /data/fivethreeone.db
          - name: MIGRATION_DIR
            value: /migrations
          ports:
            - containerPort: 8080
              name: http-api
          volumeMounts:
          - name: site-data
            mountPath: "/data"
            subPath: fivethreeone
          - name: config
            mountPath: "/config"
            readOnly: true
        volumes:
        - name: site-data
          # TODO: Some kind of mount for the SQLite database
        - name: config
          configMap:
            name: fivethreeone-config
          # This contains the routine.json file for your specific program.
---
apiVersion: v1
kind: Service
metadata:
  name: fivethreeone
spec:
  selector:
    app: fivethreeone
  ports:
    - name: web
      protocol: TCP
      port: 3000
      targetPort: 3000
    - name: http-api
      protocol: TCP
      port: 8080
      targetPort: 8080
```

</details>

And then deploy it behind something like Caddy with:

<details>

<summary>Caddyfile</summary>

```caddy
https://stronk.<domain> {
	encode gzip

	handle /api/* {
		reverse_proxy fivethreeone.<namespace>.svc.cluster.local:8080
	}

	handle {
		reverse_proxy fivethreeone.<namespace>.svc.cluster.local:3000
	}
}
```

</details>


## TODO

- [x] Make a "back"/edit button
  - One can now edit by tapping a previous lift
- [x] Make record button bigger, and farther away from skip
- [x] Add a feature to see similar "to failure" sets
- [x] Highlight personal records
  - Not done directly, but 'failure comparables' make it clear when you've made a new PR.
- [x] Make weight amount more prominent
- [x] Add support for offering to skip deload week
  - Will require adding new `routine.json` metadata