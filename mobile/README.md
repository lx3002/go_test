# Go Chat Mobile

Expo React Native client for the Go chat server.

## Run

1. Start the Go server from the repository root:

   ```powershell
   go run .
   ```

2. Install and start the mobile app:

   ```powershell
   cd mobile
   npm install
   npm run start
   ```

3. On a physical phone, set the server URL in the app to the LAN URL printed by the Go server, for example `http://192.168.1.20:8080`.

Use the same room name and private key on every device that should join the same private room.
