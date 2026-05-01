FROM node:22-alpine AS builder

WORKDIR /src

COPY . .

RUN if [ -f package.json ]; then \
      if [ -f package-lock.json ]; then npm ci; else npm install; fi; \
      npm run build; \
    else \
      mkdir -p dist && \
      cp "subconverter Admin.html" dist/index.html && \
      cp ./*.jsx dist/ && \
      cp -R docs dist/docs; \
    fi

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80
