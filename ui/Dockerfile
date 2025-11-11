FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install -g npm@11.6.2
COPY . .
RUN npm install
RUN npm run build

FROM nginx:1.27-alpine

# Remove default config and replace with non-root friendly version.
RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist/spa /usr/share/nginx/html

# Prepare directories for OpenShift random UID execution.
RUN mkdir -p /var/cache/nginx /var/run/nginx && \
    chgrp -R 0 /var/cache/nginx /var/run/nginx /var/log/nginx /etc/nginx /usr/share/nginx/html && \
    chmod -R g+rwx /var/cache/nginx /var/run/nginx /var/log/nginx && \
    chmod -R g+rwX /etc/nginx /usr/share/nginx/html

USER 101
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]

