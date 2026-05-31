FROM oven/bun:1.3 AS build
WORKDIR /src
ARG BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
ENV BUN_CONFIG_REGISTRY=$BUN_CONFIG_REGISTRY
COPY frontend-react/package.json frontend-react/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend-react/ ./

# 构建时环境变量（rewrites 目标地址，默认 localhost:8080）
ARG NEXT_PUBLIC_GO_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_GO_API_URL=$NEXT_PUBLIC_GO_API_URL

RUN bun run build

FROM oven/bun:1.3-slim AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static
COPY --from=build /src/public ./public
EXPOSE 3000
CMD ["bun", "run", "server.js"]
