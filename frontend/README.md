# stronk frontend

A barebones SvelteKit/TypeScript frontend that allows viewing + recording lifts, and setting training maxes.

## Developing

Once you've installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```bash
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a more 'production' version of app:

```bash
npm run build:dev
```

You can preview the production build with `npm run preview`.

## Deploying

To build the actual production version of the app, run:

```bash
npm run build:cloud
```

This is kind of a misnomer, as it uses the default Node adapter and can then be packaged up into a Docker image with:

```bash
docker build -t <registry host>/stronk-fe .
```

Personally, I deploy it on a homelab k8s cluster, but this same image should be fine to deploy on any cloud provider that can run Docker image (e.g. AWS Lambda or Fargate, GCP Cloud Run or Functions or App Engine Flex, Azure App Services, etc). See [the main README](/README.md) for more deployment details.
