# Minimal HTTP fixture to verify Elixir -> Docker -> Traefik without Hex deps.
# This is a bounded test server, not a replacement for Phoenix/Bandit in production.
defmodule NearprodFixture do
  @body ~s({"message":"nearprod-elixir","language":"Elixir"})
  def run do
    port = String.to_integer(System.get_env("PORT", "4000"))
    {:ok, listener} = :gen_tcp.listen(port, [:binary, packet: :raw, active: false,
      reuseaddr: true, ip: {0, 0, 0, 0}, backlog: 32])
    IO.puts("nearprod-elixir HTTP listening on 0.0.0.0:#{port}")
    accept(listener)
  end
  defp accept(listener) do
    case :gen_tcp.accept(listener) do
      {:ok, socket} ->
        # Serial bounded handling is intentional: tiny diagnostic fixture only.
        with {:ok, headers} <- headers(socket, ""),
             [method, uri | _] <- String.split(headers, " "),
             true <- method == "GET" and uri in ["/", "/health"] do
          send_response(socket, "200 OK", @body)
        else
          _ -> send_response(socket, "404 Not Found", ~s({"error":"not_found"}))
        end
        :gen_tcp.close(socket)
        accept(listener)
      {:error, :closed} -> :ok
      {:error, reason} -> raise "accept failed: #{inspect(reason)}"
    end
  end
  defp headers(socket, accumulated) when byte_size(accumulated) < 16_384 do
    if String.contains?(accumulated, "\r\n\r\n") do
      {:ok, accumulated}
    else
      case :gen_tcp.recv(socket, 0, 2_000) do
        {:ok, bytes} -> headers(socket, accumulated <> bytes)
        error -> error
      end
    end
  end
  defp headers(_, _), do: {:error, :too_large}
  defp send_response(socket, status, body) do
    :gen_tcp.send(socket, "HTTP/1.1 #{status}\r\nContent-Type: application/json\r\nContent-Length: #{byte_size(body)}\r\nConnection: close\r\n\r\n#{body}")
    IO.puts("nearprod-elixir #{status}")
  end
end
NearprodFixture.run()
