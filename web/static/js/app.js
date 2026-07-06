/* YouSee Musik – client runtime.
 * Responsibilities (only): open WebSocket, reconnect, receive JSON events,
 * update DOM, animate progress bar + equalizer. No business logic lives here.
 */
(function () {
    "use strict";

    // ---------------------------------------------------------------- state
    const prog = { positionMs: 0, durationMs: 0, playing: false, base: 0, baseAt: 0 };
    const audio = document.getElementById("player-audio");
    let hls = null;
    let currentStreamUrl = "";

    // ------------------------------------------------------------- helpers
    function fmtMs(ms) {
        if (!ms || ms < 0) ms = 0;
        const total = Math.floor(ms / 1000);
        const m = Math.floor(total / 60);
        const s = total % 60;
        return m + ":" + (s < 10 ? "0" : "") + s;
    }
    function $(id) { return document.getElementById(id); }
    function setText(id, txt) { const el = $(id); if (el) el.textContent = txt; }

    // ------------------------------------------------------- progress anim
    function setProgress(positionMs, durationMs, playing) {
        prog.positionMs = positionMs || 0;
        if (durationMs !== undefined && durationMs !== null) prog.durationMs = durationMs;
        prog.playing = !!playing;
        prog.base = prog.positionMs;
        prog.baseAt = performance.now();
    }
    function renderProgress() {
        let pos = prog.base;
        if (prog.playing) pos += performance.now() - prog.baseAt;
        if (prog.durationMs > 0 && pos > prog.durationMs) pos = prog.durationMs;
        const pct = prog.durationMs > 0 ? (pos / prog.durationMs) * 100 : 0;
        const fill = $("progress-fill");
        const knob = $("progress-knob");
        if (fill) fill.style.width = pct + "%";
        if (knob) knob.style.left = pct + "%";
        setText("time-current", fmtMs(pos));
        requestAnimationFrame(renderProgress);
    }
    requestAnimationFrame(renderProgress);

    // ------------------------------------------------------- DOM updates
    function trackImage(t) {
        return t && t.image ? t.image : "/static/img/placeholder.svg";
    }
    function artistNames(t) {
        if (!t || !t.artists) return "";
        return t.artists.map(a => a.name).join(", ");
    }

    function applyState(s) {
        const t = s.currentTrack;
        // Playbar mini info
        const pbArt = $("pb-art");
        if (pbArt) pbArt.src = trackImage(t);
        setText("pb-title", t ? t.title : "—");
        setText("pb-artist", t ? artistNames(t) : "");
        setText("time-total", t ? fmtMs((t.duration || 0) * 1000) : "0:00");

        // Now Playing main info (only if that page is mounted)
        if ($("np-title") && t) {
            setText("np-title", t.title);
            setText("np-artist", artistNames(t));
            setText("np-album", t.album ? t.album.name + (t.album.year ? " · " + t.album.year : "") : "");
            const npArt = $("np-art");
            if (npArt) npArt.src = trackImage(t);
        }

        applyPlaying(s.playing);
        applyControls(s);
        applyVolume(s);
        updateAudio(s);
    }

    function applyPlaying(playing) {
        const btn = $("btn-play");
        if (btn) btn.classList.toggle("playing", !!playing);
        const npArt = $("np-art");
        if (npArt) npArt.classList.toggle("playing", !!playing);
        const npEq = $("np-equalizer");
        if (npEq) npEq.classList.toggle("paused", !playing);
        const badge = document.querySelector(".queue-eq .equalizer");
        if (badge) badge.classList.toggle("paused", !playing);
    }

    function applyControls(s) {
        const sh = $("btn-shuffle");
        if (sh) sh.classList.toggle("active", !!s.shuffle);
        const rp = $("btn-repeat");
        if (rp) {
            rp.dataset.repeat = s.repeat;
            rp.classList.toggle("active", s.repeat && s.repeat !== "off");
        }
    }

    function applyVolume(s) {
        const slider = $("vol-slider");
        if (slider && document.activeElement !== slider) slider.value = s.muted ? 0 : s.volume;
        const mute = $("btn-mute");
        if (mute) mute.classList.toggle("muted", !!s.muted || s.volume === 0);
        if (audio) {
            audio.volume = s.muted ? 0 : (s.volume / 100);
            audio.muted = !!s.muted;
        }
    }

    function applyConnection(connected) {
        const dot = $("conn-indicator");
        if (dot) dot.dataset.connected = String(!!connected);
        setText("conn-text", connected ? "Connected" : "Disconnected");
    }

    // ------------------------------------------------------- audio (HLS)
    function updateAudio(s) {
        if (!audio) return;
        const url = s.streamUrl || "";
        if (url === currentStreamUrl) {
            syncPlayPause(s.playing);
            return;
        }
        currentStreamUrl = url;
        if (hls) { hls.destroy(); hls = null; }
        if (!url) { audio.removeAttribute("src"); audio.load(); return; }

        if (url.indexOf(".m3u8") !== -1 && window.Hls && window.Hls.isSupported()) {
            hls = new window.Hls();
            hls.loadSource(url);
            hls.attachMedia(audio);
        } else {
            audio.src = url;
        }
        syncPlayPause(s.playing);
    }
    function syncPlayPause(playing) {
        if (!audio || !currentStreamUrl) return;
        if (playing) { audio.play().catch(() => {}); }
        else { audio.pause(); }
    }

    // Move the audio element to the given position (ms). If the media isn't
    // ready yet, defer the seek until metadata has loaded.
    function seekAudio(ms) {
        if (!audio || !currentStreamUrl) return;
        const secs = ms / 1000;
        if (audio.readyState >= 1 && isFinite(audio.duration)) {
            try { audio.currentTime = secs; } catch (e) {}
        } else {
            const onReady = () => {
                audio.removeEventListener("loadedmetadata", onReady);
                try { audio.currentTime = secs; } catch (e) {}
            };
            audio.addEventListener("loadedmetadata", onReady);
        }
    }

    // ------------------------------------------------------- HTMX reloads
    function reloadQueue() {
        if (window.htmx) window.htmx.ajax("GET", "/partials/queue", { target: "#queue-panel", swap: "innerHTML" });
    }
    function reloadNowPlayingIfActive() {
        const page = document.querySelector("#main-content [data-page]");
        if (page && page.getAttribute("data-page") === "nowplaying" && window.htmx) {
            window.htmx.ajax("GET", "/partials/nowplaying", { target: "#main-content", swap: "innerHTML" });
        }
    }

    // ------------------------------------------------------- WebSocket
    let ws = null;
    let reconnectDelay = 1000;
    function connect() {
        const proto = location.protocol === "https:" ? "wss" : "ws";
        ws = new WebSocket(proto + "://" + location.host + "/ws");

        ws.onopen = function () { reconnectDelay = 1000; applyConnection(true); };
        ws.onclose = function () {
            applyConnection(false);
            setTimeout(connect, reconnectDelay);
            reconnectDelay = Math.min(reconnectDelay * 1.6, 10000);
        };
        ws.onerror = function () { ws.close(); };
        ws.onmessage = function (evt) {
            let msg;
            try { msg = JSON.parse(evt.data); } catch (e) { return; }
            handleEvent(msg.type, msg.payload);
        };
    }

    function handleEvent(type, p) {
        switch (type) {
            case "ProgressUpdated":
                setProgress(p.positionMs, p.durationMs, p.playing);
                break;
            case "PlaybackStateChanged":
                setProgress(p.positionMs, p.currentTrack ? p.currentTrack.duration * 1000 : 0, p.playing);
                applyPlaying(p.playing);
                applyControls(p);
                syncPlayPause(p.playing);
                break;
            case "TrackChanged":
                setProgress(p.positionMs, p.currentTrack ? p.currentTrack.duration * 1000 : 0, p.playing);
                applyState(p);
                reloadQueue();
                reloadNowPlayingIfActive();
                break;
            case "QueueChanged":
                reloadQueue();
                break;
            case "VolumeChanged":
                applyVolume(p);
                break;
            case "ConnectionStateChanged":
                applyConnection(p.connected);
                break;
        }
    }

    // ------------------------------------------------------- UI wiring
    function post(url) {
        fetch(url, { method: "POST" }).catch(() => {});
    }

    // Seek by clicking/dragging the progress bar.
    const bar = $("progress-bar");
    if (bar) {
        const seekAt = (clientX) => {
            const rect = bar.getBoundingClientRect();
            let frac = (clientX - rect.left) / rect.width;
            frac = Math.max(0, Math.min(1, frac));
            const ms = Math.round(frac * prog.durationMs);
            setProgress(ms, prog.durationMs, prog.playing);
            // Move the actual audio element (playback happens in the browser).
            seekAudio(ms);
            fetch("/api/seek", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ positionMs: ms }),
            }).catch(() => {});
        };
        bar.addEventListener("click", (e) => seekAt(e.clientX));
    }

    // Volume slider.
    const vol = $("vol-slider");
    if (vol) {
        let volTimer = null;
        vol.addEventListener("input", () => {
            clearTimeout(volTimer);
            const v = parseInt(vol.value, 10);
            volTimer = setTimeout(() => {
                fetch("/api/volume", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ volume: v }),
                }).catch(() => {});
            }, 80);
        });
    }

    // Sidebar active-nav toggle.
    document.addEventListener("click", (e) => {
        const nav = e.target.closest(".nav-item");
        if (nav) {
            document.querySelectorAll(".nav-item").forEach(n => n.classList.remove("active"));
            nav.classList.add("active");
        }
    });

    // Global helpers referenced from templates.
    window.ysToggleFullscreen = function () {
        if (!document.fullscreenElement) document.documentElement.requestFullscreen().catch(() => {});
        else document.exitFullscreen().catch(() => {});
    };
    window.ysCloseModal = function (e) {
        if (e && e.target && !e.target.classList.contains("modal-backdrop") && e.type === "click") {
            // called from close button (no backdrop match) – still close
        }
        const m = $("modal");
        if (m) m.innerHTML = "";
    };
    window.ysShare = function (url) {
        if (!url) { toast("No share link available"); return; }
        if (navigator.share) { navigator.share({ url: url }).catch(() => {}); }
        else if (navigator.clipboard) { navigator.clipboard.writeText(url); toast("Link copied", true); }
    };

    function toast(text, accent) {
        const c = $("toast");
        if (!c) return;
        const el = document.createElement("div");
        el.className = "toast" + (accent ? " accent" : "");
        el.textContent = text;
        c.appendChild(el);
        setTimeout(() => el.remove(), 2600);
    }
    window.ysToast = toast;

    // Toast feedback for queue actions (favorite toasts are handled by ysToggleFavorite).
    document.body.addEventListener("htmx:afterRequest", (e) => {
        const el = e.detail.elt;
        if (!el) return;
        const path = (e.detail.requestConfig && e.detail.requestConfig.path) || "";
        if (path.indexOf("/api/favorite") !== -1) toast("Added to favorites", true);
        else if (path.indexOf("/api/queue/add") !== -1) toast("Added to queue", true);
    });

    // ------------------------------------------------------- favorites
    // Favorite state is tracked client-side (by track id) so the Now Playing
    // heart stays correct across page navigation and when the track changes,
    // and can be toggled back off.
    const FAV_KEY = "ys_favorites";
    function loadFavorites() {
        try { return new Set(JSON.parse(localStorage.getItem(FAV_KEY) || "[]")); }
        catch (e) { return new Set(); }
    }
    function saveFavorites() {
        try { localStorage.setItem(FAV_KEY, JSON.stringify(Array.from(favorites))); }
        catch (e) { /* ignore (private mode / quota) */ }
    }
    const favorites = loadFavorites();

    function setFavoriteButtonState(btn, isFav) {
        btn.dataset.favorited = isFav ? "true" : "false";
        btn.classList.toggle("is-favorited", isFav);
    }
    function syncFavoriteButton() {
        const btn = $("np-favorite-btn");
        if (!btn) return;
        const trackId = btn.dataset.trackId || "";
        setFavoriteButtonState(btn, !!trackId && favorites.has(trackId));
    }
    window.ysToggleFavorite = function (btn) {
        const trackId = btn.dataset.trackId;
        if (!trackId) return;
        const willFavorite = !favorites.has(trackId);
        if (willFavorite) favorites.add(trackId); else favorites.delete(trackId);
        saveFavorites();
        setFavoriteButtonState(btn, willFavorite);
        toast(willFavorite ? "Added to favorites" : "Removed from favorites", true);
        fetch("/api/favorite", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "mediaType=Track&id=" + encodeURIComponent(trackId) + "&favorite=" + willFavorite,
        }).catch(() => {});
    };

    // Re-sync the Now Playing heart whenever its markup is (re)inserted, e.g.
    // navigating back to the page or the track changing underneath it.
    document.body.addEventListener("htmx:afterSwap", syncFavoriteButton);
    syncFavoriteButton();

    connect();
})();

