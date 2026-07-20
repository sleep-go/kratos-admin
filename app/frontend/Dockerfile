FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable
COPY app/frontend/package.json app/frontend/pnpm-lock.yaml app/frontend/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY app/frontend ./
RUN pnpm build

FROM nginx:1.29-alpine
COPY app/frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
