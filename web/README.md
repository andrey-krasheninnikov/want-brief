# Want Brief web app

The web app uses React 19, TypeScript, Vite, and Bun. It calls the Rust API through the Vite development proxy.

From the repository root, start PostgreSQL and the API with `make up` and `make api`. Then install dependencies and start the web app:

```sh
cd web
bun install --frozen-lockfile
bun run dev
```

`make web` from the repository root starts the same app with the API proxy set to <http://127.0.0.1:8080> and the web port set to 5173. For the scratch stack, use `make api-dev` and `make web-dev` in separate terminals; its ports are 8081 and 5174.

## Checks

```sh
bun test
bun run typecheck
bun run lint
bun run build
```

Routes and authentication gating are defined in `src/App.tsx`. Shared UI components are in `src/shared/`; feature screens are in `src/features/`. The typed API client is `src/api.ts`.
