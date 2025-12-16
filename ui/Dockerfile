FROM node:22-alpine AS build
ARG BUILD_VERSION
ARG BUILD_NUMBER
ARG BUILD_SHA
WORKDIR /app
COPY package*.json ./
# Cache npm global install and configure npm for faster installs
RUN --mount=type=cache,target=/root/.npm \
    npm install -g npm@11.6.2 && \
    npm config set audit false && \
    npm config set fund false && \
    npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm config set maxsockets 50
COPY . .
# Use npm ci with BuildKit cache mount for faster repeat builds
# If package-lock.json is out of sync, fall back to npm install
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund || \
    (echo "⚠️  package-lock.json out of sync, using npm install instead..." && \
     npm install --no-audit --no-fund)
# Inject build version and SHA into Vite env
ENV VITE_BUILD_VERSION=${BUILD_VERSION}
ENV VITE_BUILD_NUMBER=${BUILD_NUMBER}
ENV VITE_BUILD_SHA=${BUILD_SHA}
RUN npm run build

# Stage 2: Serve the app with Node.js serve
FROM node:22-alpine

# Configure npm and install serve globally (with cache for faster rebuilds)
RUN npm config set audit false && \
    npm config set fund false
RUN --mount=type=cache,target=/root/.npm \
    npm install -g serve@14.2.1

# Create app directory and copy built files
WORKDIR /app
COPY --from=build /app/dist/spa ./dist/spa

# Copy entrypoint script to inject runtime config
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Prepare directories for OpenShift random UID execution
RUN chgrp -R 0 /app && \
    chmod -R g+rwX /app

USER 1001
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]

