FROM node:22-alpine
WORKDIR /app
COPY --chown=node:node app.mjs ./app.mjs
USER node
EXPOSE 3000
CMD ["node", "app.mjs"]
