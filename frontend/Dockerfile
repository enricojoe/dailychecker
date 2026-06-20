# syntax=docker/dockerfile:1

# ── Build stage ──────────────────────────────────────────────────────────────
FROM node:22-alpine AS build
WORKDIR /app

# Install deps against the lockfile for reproducible builds.
COPY package.json package-lock.json ./
RUN npm ci

# Build the SPA. VITE_API_BASE_URL is baked in at build time (Vite inlines
# VITE_* vars), so it must be provided as a build arg for production images.
ARG VITE_API_BASE_URL=http://localhost:8080/api
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
COPY . .
RUN npm run build

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM nginx:1.27-alpine AS runtime

# SPA routing: serve index.html for unknown paths so client-side routes work.
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
