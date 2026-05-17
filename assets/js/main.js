const TOKEN_KEY = "aurora_token";
const USER_KEY = "aurora_user";
const POLL_INTERVAL_MS = 2000;

const state = {
  token: "",
  user: null,
  pollingHandle: null,
  thumbnailCache: new Map(),
  authMode: "register",
  activeStreamURL: "",
  modalOpen: false,
};

function byId(id) {
  return document.getElementById(id);
}

function setMessage(element, text, type) {
  element.textContent = text || "";
  element.classList.remove("is-error", "is-success");
  if (type === "error") {
    element.classList.add("is-error");
  } else if (type === "success") {
    element.classList.add("is-success");
  }
}

function stopPolling() {
  if (state.pollingHandle) {
    clearInterval(state.pollingHandle);
    state.pollingHandle = null;
  }
}

function renderUserInfo() {
  const userInfo = byId("userInfo");
  const email = state.user && state.user.email ? state.user.email : "Authenticated";
  userInfo.textContent = `Signed in as ${email}`;
}

function persistAuth(token, user) {
  state.token = token;
  state.user = user || null;
  localStorage.setItem(TOKEN_KEY, token);
  if (user) {
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  }
}

function clearAuth() {
  for (const url of state.thumbnailCache.values()) {
    URL.revokeObjectURL(url);
  }
  state.thumbnailCache.clear();
  state.activeStreamURL = "";
  state.token = "";
  state.user = null;
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

function closeVideoModal() {
  const modal = byId("videoModal");
  const player = byId("videoPlayer");
  if (!modal || !player) {
    return;
  }
  player.pause();
  player.removeAttribute("src");
  player.load();
  if (state.activeStreamURL && state.activeStreamURL.startsWith("blob:")) {
    URL.revokeObjectURL(state.activeStreamURL);
  }
  state.activeStreamURL = "";
  state.modalOpen = false;
  if (modal.open) {
    modal.close();
  }
  if (state.token) {
    startPolling();
  }
}

function setViewAuthenticated(isAuthenticated) {
  const authSection = byId("authSection");
  const appSection = byId("appSection");
  if (isAuthenticated) {
    authSection.classList.add("hidden");
    appSection.classList.remove("hidden");
    renderUserInfo();
    startPolling();
  } else {
    stopPolling();
    appSection.classList.add("hidden");
    authSection.classList.remove("hidden");
    byId("videosList").innerHTML = "";
  }
}

function setAuthMode(mode) {
  state.authMode = mode === "login" ? "login" : "register";
  const registerForm = byId("registerForm");
  const loginForm = byId("loginForm");
  const authTitle = byId("authTitle");
  const authMessage = byId("authMessage");

  if (state.authMode === "login") {
    registerForm.classList.add("hidden");
    loginForm.classList.remove("hidden");
    authTitle.textContent = "Welcome back";
    authMessage.textContent = "";
    byId("loginEmail").focus();
    return;
  }

  loginForm.classList.add("hidden");
  registerForm.classList.remove("hidden");
  authTitle.textContent = "Create your account";
  authMessage.textContent = "";
  byId("registerEmail").focus();
}

function formatDate(value) {
  if (!value) {
    return "unknown time";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function escapeHtml(input) {
  return String(input)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function cleanupThumbnailCache(videos) {
  const keep = new Set(
    (videos || [])
      .filter((video) => String(video.status || "").toLowerCase() === "ready")
      .map((video) => String(video.id || "")),
  );

  for (const [videoID, url] of state.thumbnailCache.entries()) {
    if (!keep.has(videoID)) {
      URL.revokeObjectURL(url);
      state.thumbnailCache.delete(videoID);
    }
  }
}

async function loadThumbnail(videoID) {
  if (!state.token || !videoID || state.thumbnailCache.has(videoID)) {
    return;
  }

  const response = await fetch(`/api/videos/${encodeURIComponent(videoID)}/thumbnail`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401) {
    handleUnauthorized();
    return;
  }
  if (!response.ok) {
    return;
  }

  const blob = await response.blob();
  const objectURL = URL.createObjectURL(blob);
  state.thumbnailCache.set(videoID, objectURL);
}

function applyThumbnailToDom(videoID) {
  const thumbURL = state.thumbnailCache.get(videoID);
  if (!thumbURL) {
    return;
  }
  const images = document.querySelectorAll(`[data-thumbnail-for="${videoID}"]`);
  for (const image of images) {
    image.src = thumbURL;
  }
}

async function hydrateThumbnails(videos) {
  const readyVideos = (videos || []).filter((video) => String(video.status || "").toLowerCase() === "ready");
  for (const video of readyVideos) {
    const videoID = String(video.id || "");
    if (!videoID) {
      continue;
    }
    try {
      await loadThumbnail(videoID);
      applyThumbnailToDom(videoID);
    } catch (_err) {
      // Thumbnail loading is best-effort; UI remains functional without it.
    }
  }
}

async function downloadProcessed(videoID, filename) {
  if (!state.token) {
    handleUnauthorized();
    return;
  }

  const response = await fetch(`/api/videos/${encodeURIComponent(videoID)}/download`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401) {
    handleUnauthorized();
    return;
  }
  if (!response.ok) {
    setMessage(byId("appMessage"), "Download fehlgeschlagen.", "error");
    return;
  }

  const blob = await response.blob();
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectURL;
  anchor.download = `${filename || "video"}-720p.mp4`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}

function bindDownloadButtons() {
  const buttons = document.querySelectorAll("[data-action='download-video']");
  for (const button of buttons) {
    button.addEventListener("click", () => {
      const videoID = button.getAttribute("data-video-id");
      const filename = button.getAttribute("data-filename");
      downloadProcessed(videoID, filename).catch(() => {
        setMessage(byId("appMessage"), "Download fehlgeschlagen.", "error");
      });
    });
  }
}

async function requestStreamURL(videoID) {
  const { response, data } = await jsonRequest(`/api/videos/${encodeURIComponent(videoID)}/stream`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401) {
    handleUnauthorized();
    return "";
  }
  if (!response.ok) {
    const message = data && data.error && data.error.message ? data.error.message : "Stream konnte nicht gestartet werden.";
    setMessage(byId("appMessage"), message, "error");
    return "";
  }

  return data && typeof data.stream_url === "string" ? data.stream_url : "";
}

async function fetchPlaybackBlobURL(videoID) {
  const response = await fetch(`/api/videos/${encodeURIComponent(videoID)}/download`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401) {
    handleUnauthorized();
    return "";
  }
  if (!response.ok) {
    return "";
  }

  const blob = await response.blob();
  return URL.createObjectURL(blob);
}

function isBrowserReachableMediaURL(rawURL) {
  if (!rawURL || rawURL.startsWith("blob:")) {
    return true;
  }
  try {
    const parsed = new URL(rawURL, window.location.origin);
    if (parsed.hostname === "rustfs") {
      return false;
    }
    return parsed.hostname === window.location.hostname
      || parsed.hostname === "localhost"
      || parsed.hostname === "127.0.0.1";
  } catch (_err) {
    return false;
  }
}

async function playInBrowser(videoID, filename) {
  if (!state.token) {
    handleUnauthorized();
    return;
  }

  let streamURL = await fetchPlaybackBlobURL(videoID);
  if (!streamURL) {
    const presigned = await requestStreamURL(videoID);
    if (presigned && isBrowserReachableMediaURL(presigned)) {
      streamURL = presigned;
    }
  }
  if (!streamURL) {
    setMessage(byId("appMessage"), "Video konnte nicht geladen werden.", "error");
    return;
  }

  const modal = byId("videoModal");
  const player = byId("videoPlayer");
  const modalTitle = byId("videoModalTitle");
  stopPolling();
  state.modalOpen = true;
  modalTitle.textContent = filename ? `Playback: ${filename}` : "Video playback";
  player.src = streamURL;
  state.activeStreamURL = streamURL;
  modal.showModal();
  await player.play().catch(() => {});
}

function bindPlayButtons() {
  const buttons = document.querySelectorAll("[data-action='play-video']");
  for (const button of buttons) {
    button.addEventListener("click", () => {
      const videoID = button.getAttribute("data-video-id");
      const filename = button.getAttribute("data-filename");
      playInBrowser(videoID, filename).catch(() => {
        setMessage(byId("appMessage"), "Stream konnte nicht gestartet werden.", "error");
      });
    });
  }
}

function renderVideos(videos) {
  const list = byId("videosList");
  if (!Array.isArray(videos) || videos.length === 0) {
    list.innerHTML = `<div class="empty-state">No videos yet. Upload one to start processing.</div>`;
    return;
  }

  const sorted = [...videos].sort((a, b) => {
    const aTs = new Date(a.created_at || 0).getTime();
    const bTs = new Date(b.created_at || 0).getTime();
    return bTs - aTs;
  });
  cleanupThumbnailCache(sorted);

  list.innerHTML = sorted
    .map((video) => {
      const status = (video.status || "uploaded").toLowerCase();
      const badgeClass = `status-${status}`;
      const filename = escapeHtml(video.filename || "unnamed-file");
      const videoID = String(video.id || "");
      const downloadAction =
        status === "ready" && videoID
          ? `<button class="download-link" data-action="download-video" data-video-id="${videoID}" data-filename="${filename}" type="button">Download 720p</button>`
          : `<span class="download-link disabled">Download verfügbar, sobald ready</span>`;
      const playAction =
        status === "ready" && videoID
          ? `<button class="play-link" data-action="play-video" data-video-id="${videoID}" data-filename="${filename}" type="button">Play im Browser</button>`
          : "";
      return `
        <article class="video-item">
          <img
            class="thumb"
            data-thumbnail-for="${videoID}"
            src=""
            alt="Thumbnail for ${filename}"
          />
          <div class="video-meta">
            <div class="name" title="${filename}">${filename}</div>
            <div class="sub">Created: ${formatDate(video.created_at)}</div>
            <div class="sub">${downloadAction}</div>
            <div class="sub">${playAction}</div>
          </div>
          <span class="status-badge ${badgeClass}">${status}</span>
        </article>
      `;
    })
    .join("");

  bindDownloadButtons();
  bindPlayButtons();
  hydrateThumbnails(sorted);
}

function handleUnauthorized() {
  closeVideoModal();
  clearAuth();
  setViewAuthenticated(false);
  setAuthMode("login");
  setMessage(byId("appMessage"), "Session expired. Please login again.", "error");
}

async function jsonRequest(url, options = {}) {
  const response = await fetch(url, options);
  let data = null;
  try {
    data = await response.json();
  } catch (_err) {
    data = null;
  }
  return { response, data };
}

async function fetchVideos(silent = false) {
  if (!state.token) {
    return;
  }

  const { response, data } = await jsonRequest("/api/videos?limit=50&page=1", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401) {
    handleUnauthorized();
    return;
  }
  if (!response.ok) {
    if (!silent) {
      setMessage(byId("appMessage"), "Failed to load videos.", "error");
    }
    return;
  }

  const videos = data && Array.isArray(data.videos) ? data.videos : [];
  renderVideos(videos);
}

function startPolling() {
  if (state.modalOpen) {
    return;
  }
  stopPolling();
  fetchVideos(true).catch(() => {
    setMessage(byId("appMessage"), "Failed to load videos.", "error");
  });
  state.pollingHandle = setInterval(() => {
    fetchVideos(true).catch(() => {
      setMessage(byId("appMessage"), "Failed to refresh videos.", "error");
    });
  }, POLL_INTERVAL_MS);
}

async function submitAuth(mode, form) {
  const formData = new FormData(form);
  const email = String(formData.get("email") || "").trim();
  const password = String(formData.get("password") || "").trim();

  if (!email || !password) {
    setMessage(byId("authMessage"), "Please provide email and password.", "error");
    return;
  }

  const endpoint = mode === "register" ? "/api/auth/register" : "/api/auth/login";
  setMessage(byId("authMessage"), mode === "register" ? "Creating account..." : "Signing in...", "");

  const { response, data } = await jsonRequest(endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ email, password }),
  });

  if (!response.ok) {
    const message = data && data.error && data.error.message ? data.error.message : "Authentication failed.";
    setMessage(byId("authMessage"), message, "error");
    return;
  }

  const token = data && data.token;
  const user = data && data.user;
  if (!token) {
    setMessage(byId("authMessage"), "No token in auth response.", "error");
    return;
  }

  persistAuth(token, user);
  setMessage(byId("authMessage"), mode === "register" ? "Account created. Logged in." : "Login successful.", "success");
  setMessage(byId("appMessage"), "Authenticated.", "success");
  setViewAuthenticated(true);
}

async function handleUpload(event) {
  event.preventDefault();
  if (!state.token) {
    handleUnauthorized();
    return;
  }

  const fileInput = byId("fileInput");
  const uploadButton = byId("uploadButton");
  const file = fileInput.files && fileInput.files[0];

  if (!file) {
    setMessage(byId("appMessage"), "Please choose a video file first.", "error");
    return;
  }

  uploadButton.disabled = true;
  setMessage(byId("appMessage"), "Uploading...", "");

  const payload = new FormData();
  payload.append("file", file);

  const response = await fetch("/api/upload", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
    body: payload,
  });

  let data = null;
  try {
    data = await response.json();
  } catch (_err) {
    data = null;
  }

  uploadButton.disabled = false;

  if (response.status === 401) {
    handleUnauthorized();
    return;
  }
  if (!response.ok) {
    const message = data && data.error && data.error.message ? data.error.message : "Upload failed.";
    setMessage(byId("appMessage"), message, "error");
    return;
  }

  fileInput.value = "";
  setMessage(byId("appMessage"), "Upload accepted. Processing started.", "success");
  await fetchVideos(true);
}

function handleLogout() {
  clearAuth();
  setViewAuthenticated(false);
  setAuthMode("login");
  setMessage(byId("authMessage"), "Logged out.", "success");
  setMessage(byId("appMessage"), "", "");
}

async function bootstrapAuthenticatedSession() {
  const rawToken = localStorage.getItem(TOKEN_KEY);
  if (!rawToken) {
    setViewAuthenticated(false);
    return;
  }

  state.token = rawToken;
  const rawUser = localStorage.getItem(USER_KEY);
  if (rawUser) {
    try {
      state.user = JSON.parse(rawUser);
    } catch (_err) {
      state.user = null;
    }
  }

  const { response, data } = await jsonRequest("/api/users/me", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${state.token}`,
    },
  });

  if (response.status === 401 || !response.ok) {
    handleUnauthorized();
    return;
  }

  if (data && data.user) {
    state.user = data.user;
    localStorage.setItem(USER_KEY, JSON.stringify(data.user));
  }

  setViewAuthenticated(true);
}

document.addEventListener("DOMContentLoaded", () => {
  const loginForm = byId("loginForm");
  const registerForm = byId("registerForm");
  const uploadForm = byId("uploadForm");
  const logoutButton = byId("logoutButton");
  const showLoginLink = byId("showLoginLink");
  const showRegisterLink = byId("showRegisterLink");
  const closeVideoModalButton = byId("closeVideoModalButton");
  const videoModal = byId("videoModal");

  setAuthMode("register");

  showLoginLink.addEventListener("click", () => {
    setAuthMode("login");
    setMessage(byId("authMessage"), "", "");
  });

  showRegisterLink.addEventListener("click", () => {
    setAuthMode("register");
    setMessage(byId("authMessage"), "", "");
  });

  loginForm.addEventListener("submit", (event) => {
    event.preventDefault();
    submitAuth("login", loginForm).catch(() => {
      setMessage(byId("authMessage"), "Login request failed.", "error");
    });
  });

  registerForm.addEventListener("submit", (event) => {
    event.preventDefault();
    submitAuth("register", registerForm).catch(() => {
      setMessage(byId("authMessage"), "Register request failed.", "error");
    });
  });

  uploadForm.addEventListener("submit", (event) => {
    handleUpload(event).catch(() => {
      setMessage(byId("appMessage"), "Upload request failed.", "error");
    });
  });

  logoutButton.addEventListener("click", handleLogout);
  closeVideoModalButton.addEventListener("click", closeVideoModal);
  videoModal.addEventListener("cancel", (event) => {
    event.preventDefault();
    closeVideoModal();
  });

  bootstrapAuthenticatedSession().catch(() => {
    handleUnauthorized();
  });
});
