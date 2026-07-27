FROM node:20-alpine

RUN apk add --no-cache docker-cli docker-cli-compose git

WORKDIR /app

COPY ui/package*.json ./
RUN npm ci

COPY ui ./
RUN npm run build

EXPOSE 5055

CMD ["node", "server.mjs"]
