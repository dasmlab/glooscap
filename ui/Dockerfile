FROM node:22-alpine AS build
ARG BUILD_VERSION
ARG BUILD_NUMBER
WORKDIR /app
COPY package*.json ./
RUN npm install -g npm@11.6.2
COPY . .
RUN npm install
# Inject build version into Vite env
ENV VITE_BUILD_VERSION=${BUILD_VERSION}
ENV VITE_BUILD_NUMBER=${BUILD_NUMBER}
RUN npm run build

# Stage 2: Serve the app with Node.js serve
FROM node:22-alpine

# Install serve globally for serving static files
RUN npm install -g serve@14.2.1

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

