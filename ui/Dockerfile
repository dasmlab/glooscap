FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist/spa /usr/share/nginx/html
RUN printf 'server { listen 8080;\n  root /usr/share/nginx/html;\n  include /etc/nginx/mime.types;\n  try_files $uri $uri/ /index.html;\n}\n' > /etc/nginx/conf.d/default.conf
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]

