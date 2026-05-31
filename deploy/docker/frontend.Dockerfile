FROM oven/bun:1.3 AS build
WORKDIR /src
ARG BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
ENV BUN_CONFIG_REGISTRY=$BUN_CONFIG_REGISTRY
COPY frontend-react/package.json frontend-react/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend-react/ ./
RUN bun run build

FROM oven/bun:1.3-slim AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static
COPY --from=build /src/public ./public
EXPOSE 3000
CMD ["bun", "run", "server.js"]
