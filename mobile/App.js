import React, { useEffect, useRef, useState } from "react";
import {
  FlatList,
  Image,
  KeyboardAvoidingView,
  Linking,
  Platform,
  Pressable,
  SafeAreaView,
  StyleSheet,
  Text,
  TextInput,
  View
} from "react-native";
import { ResizeMode, Video } from "expo-av";
import * as ImagePicker from "expo-image-picker";

const DEFAULT_SERVER_URL = "http://localhost:8080";

export default function App() {
  const socketRef = useRef(null);
  const [serverUrl, setServerUrl] = useState(DEFAULT_SERVER_URL);
  const [username, setUsername] = useState("");
  const [room, setRoom] = useState("general");
  const [roomMode, setRoomMode] = useState("public");
  const [privateKey, setPrivateKey] = useState("");
  const [sendMode, setSendMode] = useState("room");
  const [dmTarget, setDmTarget] = useState("");
  const [messageText, setMessageText] = useState("");
  const [token, setToken] = useState("");
  const [connected, setConnected] = useState(false);
  const [messages, setMessages] = useState([]);
  const [error, setError] = useState("");

  useEffect(() => {
    return () => {
      if (socketRef.current) {
        socketRef.current.close();
      }
    };
  }, []);

  const connect = async () => {
    setError("");
    const cleanUsername = username.trim();
    const cleanRoom = normalizeRoom(room);
    const cleanPrivateKey = privateKey.trim();

    if (!cleanUsername) {
      setError("Display name is required.");
      return;
    }
    if (roomMode === "private" && !cleanPrivateKey) {
      setError("Private key is required.");
      return;
    }

    try {
      const loginParams = new URLSearchParams({ username: cleanUsername });
      const loginResponse = await fetch(`${cleanBaseUrl()}/login?${loginParams.toString()}`);
      if (!loginResponse.ok) {
        throw new Error(await loginResponse.text());
      }
      const loginData = await loginResponse.json();
      setToken(loginData.token);
      setUsername(loginData.username);
      setRoom(cleanRoom);

      const wsUrl = new URL(cleanBaseUrl());
      wsUrl.protocol = wsUrl.protocol === "https:" ? "wss:" : "ws:";
      wsUrl.pathname = "/ws";
      wsUrl.search = "";
      wsUrl.searchParams.set("token", loginData.token);
      wsUrl.searchParams.set("room", cleanRoom);
      wsUrl.searchParams.set("private", roomMode === "private" ? "1" : "0");
      if (roomMode === "private") {
        wsUrl.searchParams.set("room_key", cleanPrivateKey);
      }

      if (socketRef.current) {
        socketRef.current.close();
      }

      const socket = new WebSocket(wsUrl.toString());
      socketRef.current = socket;

      socket.onopen = () => {
        setConnected(true);
        setMessages([]);
      };
      socket.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data);
          setMessages((current) => [...current, { ...parsed, id: `${Date.now()}-${current.length}` }]);
        } catch {
          setMessages((current) => [
            ...current,
            {
              id: `${Date.now()}-${current.length}`,
              username: "Server",
              content: event.data,
              media_type: "text",
              type: "system",
              timestamp: new Date().toISOString()
            }
          ]);
        }
      };
      socket.onerror = () => setError("Connection error.");
      socket.onclose = () => setConnected(false);
    } catch (err) {
      setConnected(false);
      setError("Could not connect to the server.");
    }
  };

  const disconnect = () => {
    if (socketRef.current) {
      socketRef.current.close();
      socketRef.current = null;
    }
    setConnected(false);
  };

  const sendText = () => {
    const text = messageText.trim();
    if (!text) return;
    sendPayload(text, "text");
    setMessageText("");
  };

  const pickAndSendMedia = async () => {
    setError("");
    if (!connected || !token) return;
    if (sendMode === "dm" && !dmTarget.trim()) {
      setError("DM username is required.");
      return;
    }

    const permission = await ImagePicker.requestMediaLibraryPermissionsAsync();
    if (!permission.granted) {
      setError("Media permission is required.");
      return;
    }

    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ["images", "videos"],
      quality: 0.85
    });
    if (result.canceled || !result.assets || result.assets.length === 0) return;

    const asset = result.assets[0];
    const mimeType = asset.mimeType || guessMimeType(asset);
    const name = asset.fileName || `upload.${mimeType.split("/")[1] || "bin"}`;
    const formData = new FormData();
    formData.append("media", {
      uri: asset.uri,
      name,
      type: mimeType
    });

    try {
      const response = await fetch(`${cleanBaseUrl()}/upload`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`
        },
        body: formData
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const data = await response.json();
      sendPayload(data.url, data.media_type);
    } catch (err) {
      setError("Upload failed.");
    }
  };

  const sendPayload = (content, mediaType) => {
    if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) {
      setError("Socket is not connected.");
      return;
    }
    if (sendMode === "dm" && !dmTarget.trim()) {
      setError("DM username is required.");
      return;
    }

    const isDM = sendMode === "dm";
    const payload = {
      type: isDM ? "dm" : roomMode === "private" ? "private_room" : "room",
      target: isDM ? dmTarget.trim() : normalizeRoom(room),
      content,
      media_type: mediaType
    };
    socketRef.current.send(JSON.stringify(payload));
  };

  const cleanBaseUrl = () => serverUrl.trim().replace(/\/+$/, "");

  const renderMessage = ({ item }) => {
    const mine = item.username === username;
    const source = absoluteUrl(item.content, cleanBaseUrl());
    return (
      <View style={[styles.message, mine && styles.mine, item.type === "dm" && styles.dm, item.type === "private_room" && styles.privateRoom]}>
        <View style={styles.metaRow}>
          <Text style={styles.metaText}>{item.username || "Guest"}</Text>
          <Text style={styles.metaText}>{formatTime(item.timestamp)}</Text>
          {item.type === "dm" ? <Text style={styles.badge}>DM</Text> : null}
          {item.type === "private_room" ? <Text style={styles.badge}>Private</Text> : null}
        </View>
        {item.media_type === "image" ? (
          <Image source={{ uri: source }} style={styles.image} resizeMode="cover" />
        ) : item.media_type === "video" ? (
          <Video source={{ uri: source }} style={styles.video} useNativeControls resizeMode={ResizeMode.CONTAIN} />
        ) : (
          <Text style={styles.messageText}>{item.content}</Text>
        )}
      </View>
    );
  };

  return (
    <SafeAreaView style={styles.safe}>
      <KeyboardAvoidingView style={styles.app} behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <View style={styles.header}>
          <View>
            <Text style={styles.brand}>Go Chat</Text>
            <Text style={styles.subtle}>{connected ? `${roomMode}: ${room}` : "Offline"}</Text>
          </View>
          <View style={[styles.statusDot, connected && styles.statusOnline]} />
        </View>

        <View style={styles.form}>
          <TextInput value={serverUrl} onChangeText={setServerUrl} autoCapitalize="none" style={styles.input} placeholder="Server URL" editable={!connected} />
          <TextInput value={username} onChangeText={setUsername} style={styles.input} placeholder="Display name" editable={!connected} />
          <View style={styles.row}>
            <Pressable style={[styles.tab, roomMode === "public" && styles.activeTab]} disabled={connected} onPress={() => setRoomMode("public")}>
              <Text style={[styles.tabText, roomMode === "public" && styles.activeTabText]}>Public</Text>
            </Pressable>
            <Pressable style={[styles.tab, roomMode === "private" && styles.activeTab]} disabled={connected} onPress={() => setRoomMode("private")}>
              <Text style={[styles.tabText, roomMode === "private" && styles.activeTabText]}>Private</Text>
            </Pressable>
          </View>
          <TextInput value={room} onChangeText={setRoom} autoCapitalize="none" style={styles.input} placeholder="Room" editable={!connected} />
          {roomMode === "private" ? (
            <TextInput value={privateKey} onChangeText={setPrivateKey} autoCapitalize="none" secureTextEntry style={styles.input} placeholder="Private key" editable={!connected} />
          ) : null}
          <Pressable style={styles.primaryButton} onPress={connected ? disconnect : connect}>
            <Text style={styles.primaryButtonText}>{connected ? "Leave" : "Join"}</Text>
          </Pressable>
          {error ? <Text style={styles.error}>{error}</Text> : null}
        </View>

        <FlatList
          style={styles.messages}
          contentContainerStyle={messages.length === 0 ? styles.emptyMessages : styles.messageList}
          data={messages}
          keyExtractor={(item, index) => item.id || `${item.timestamp}-${index}`}
          renderItem={renderMessage}
          ListEmptyComponent={<Text style={styles.subtle}>No messages.</Text>}
        />

        <View style={styles.composer}>
          <View style={styles.row}>
            <Pressable style={[styles.tab, sendMode === "room" && styles.activeTab]} disabled={!connected} onPress={() => setSendMode("room")}>
              <Text style={[styles.tabText, sendMode === "room" && styles.activeTabText]}>Room</Text>
            </Pressable>
            <Pressable style={[styles.tab, sendMode === "dm" && styles.activeTab]} disabled={!connected} onPress={() => setSendMode("dm")}>
              <Text style={[styles.tabText, sendMode === "dm" && styles.activeTabText]}>DM</Text>
            </Pressable>
          </View>
          {sendMode === "dm" ? (
            <TextInput value={dmTarget} onChangeText={setDmTarget} autoCapitalize="none" style={styles.input} placeholder="DM username" editable={connected} />
          ) : null}
          <View style={styles.sendRow}>
            <Pressable style={[styles.mediaButton, !connected && styles.disabled]} disabled={!connected} onPress={pickAndSendMedia}>
              <Text style={styles.mediaButtonText}>Media</Text>
            </Pressable>
            <TextInput value={messageText} onChangeText={setMessageText} style={styles.messageInput} placeholder="Message" editable={connected} />
            <Pressable style={[styles.sendButton, !connected && styles.disabled]} disabled={!connected} onPress={sendText}>
              <Text style={styles.sendButtonText}>Send</Text>
            </Pressable>
          </View>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

function normalizeRoom(value) {
  return (value || "").trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "general";
}

function absoluteUrl(value, baseUrl) {
  try {
    return new URL(value, baseUrl).toString();
  } catch {
    return value;
  }
}

function guessMimeType(asset) {
  if (asset.type === "video") return "video/mp4";
  if (asset.uri && asset.uri.toLowerCase().endsWith(".png")) return "image/png";
  return "image/jpeg";
}

function formatTime(value) {
  const date = value ? new Date(value) : new Date();
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

const styles = StyleSheet.create({
  safe: {
    flex: 1,
    backgroundColor: "#eef3f5"
  },
  app: {
    flex: 1,
    padding: 14,
    gap: 12
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between"
  },
  brand: {
    color: "#172033",
    fontSize: 24,
    fontWeight: "800"
  },
  subtle: {
    color: "#647084",
    marginTop: 2
  },
  statusDot: {
    width: 14,
    height: 14,
    borderRadius: 7,
    backgroundColor: "#98a2b3"
  },
  statusOnline: {
    backgroundColor: "#12b76a"
  },
  form: {
    gap: 8
  },
  input: {
    minHeight: 42,
    borderWidth: 1,
    borderColor: "#d9e2e8",
    borderRadius: 8,
    backgroundColor: "#ffffff",
    paddingHorizontal: 12,
    color: "#172033"
  },
  row: {
    flexDirection: "row",
    gap: 8
  },
  tab: {
    flex: 1,
    minHeight: 40,
    alignItems: "center",
    justifyContent: "center",
    borderWidth: 1,
    borderColor: "#d9e2e8",
    borderRadius: 8,
    backgroundColor: "#f3f6f8"
  },
  activeTab: {
    borderColor: "#0f766e",
    backgroundColor: "#e7f8f3"
  },
  tabText: {
    color: "#647084",
    fontWeight: "700"
  },
  activeTabText: {
    color: "#0f766e"
  },
  primaryButton: {
    minHeight: 44,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 8,
    backgroundColor: "#0f766e"
  },
  primaryButtonText: {
    color: "#ffffff",
    fontWeight: "800"
  },
  error: {
    color: "#b42318"
  },
  messages: {
    flex: 1
  },
  messageList: {
    gap: 10,
    paddingBottom: 8
  },
  emptyMessages: {
    flexGrow: 1,
    alignItems: "center",
    justifyContent: "center"
  },
  message: {
    maxWidth: "86%",
    borderWidth: 1,
    borderColor: "#d9e2e8",
    borderRadius: 8,
    padding: 10,
    backgroundColor: "#ffffff",
    alignSelf: "flex-start"
  },
  mine: {
    alignSelf: "flex-end",
    backgroundColor: "#e7f8f3"
  },
  dm: {
    borderColor: "#b8c4ff",
    backgroundColor: "#edf1ff"
  },
  privateRoom: {
    borderColor: "#f1c565",
    backgroundColor: "#fff4d8"
  },
  metaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    marginBottom: 5
  },
  metaText: {
    color: "#647084",
    fontSize: 12,
    fontWeight: "700"
  },
  badge: {
    overflow: "hidden",
    borderRadius: 999,
    paddingHorizontal: 6,
    paddingVertical: 1,
    backgroundColor: "rgba(23, 32, 51, 0.08)",
    color: "#172033",
    fontSize: 11
  },
  messageText: {
    color: "#172033",
    lineHeight: 20
  },
  image: {
    width: 250,
    height: 190,
    borderRadius: 8,
    backgroundColor: "#d9e2e8"
  },
  video: {
    width: 250,
    height: 190,
    borderRadius: 8,
    backgroundColor: "#172033"
  },
  composer: {
    gap: 8
  },
  sendRow: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center"
  },
  mediaButton: {
    minHeight: 42,
    paddingHorizontal: 12,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#ffffff",
    borderWidth: 1,
    borderColor: "#d9e2e8"
  },
  mediaButtonText: {
    color: "#172033",
    fontWeight: "700"
  },
  messageInput: {
    flex: 1,
    minHeight: 42,
    borderWidth: 1,
    borderColor: "#d9e2e8",
    borderRadius: 8,
    backgroundColor: "#ffffff",
    paddingHorizontal: 12,
    color: "#172033"
  },
  sendButton: {
    minHeight: 42,
    paddingHorizontal: 14,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#0f766e"
  },
  sendButtonText: {
    color: "#ffffff",
    fontWeight: "800"
  },
  disabled: {
    opacity: 0.5
  }
});
