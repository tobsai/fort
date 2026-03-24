# Stage 1: Build
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy source (node_modules, dist, .git excluded via .dockerignore)
COPY . .

# Install all dependencies (creates node_modules + workspace symlinks)
RUN npm ci

# Verify workspace symlink was created
RUN test -L node_modules/@fort/core || (echo "ERROR: @fort/core symlink missing" && exit 1)

# Build using the installed tsc binary (not npx which may resolve differently)
RUN ./node_modules/.bin/tsc --build tsconfig.json

# Fail fast if declarations weren't emitted
RUN test -f packages/core/dist/index.d.ts || (echo "ERROR: core declarations not emitted" && exit 1)
RUN test -f packages/cli/dist/index.js || (echo "ERROR: cli not built" && exit 1)

# Build dashboard (vite, separate from tsc)
RUN npm run build --workspace=packages/dashboard

# Stage 2: Runtime
FROM node:20-bookworm-slim AS runtime

WORKDIR /app
ENV NODE_ENV=production

# Copy package files and install prod deps only
COPY package*.json ./
COPY packages/core/package*.json ./packages/core/
COPY packages/cli/package*.json ./packages/cli/
COPY packages/dashboard/package*.json ./packages/dashboard/
RUN npm ci --omit=dev

# Copy built artifacts from builder
COPY --from=builder /app/packages/core/dist ./packages/core/dist
COPY --from=builder /app/packages/cli/dist ./packages/cli/dist
COPY --from=builder /app/packages/dashboard/dist ./packages/dashboard/dist

EXPOSE ${PORT:-4077}

CMD ["sh", "-c", "node packages/cli/dist/index.js portal --port ${PORT:-4077} --no-open"]
