FROM elixir:1.18.4-otp-27-slim
WORKDIR /app
COPY server.exs ./server.exs
USER nobody
ENV PORT=4000 ERL_FLAGS="+S 2:2"
EXPOSE 4000
CMD ["elixir", "server.exs"]
