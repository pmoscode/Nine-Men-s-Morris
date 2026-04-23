'use strict';

// ─── Sound engine (Web Audio API, no external files) ─────────────────────────

function mkSounds(isMuted) {
  const AudioCtx = window.AudioContext || window.webkitAudioContext;
  let ctx = null;

  function ac() {
    if (!ctx) ctx = new AudioCtx();
    if (ctx.state === 'suspended') ctx.resume();
    return ctx;
  }

  function tone(freq, type, gain, dur, when) {
    if (isMuted()) return;
    const c = ac();
    const t = c.currentTime + (when || 0);
    const osc = c.createOscillator();
    const g   = c.createGain();
    osc.connect(g);
    g.connect(c.destination);
    osc.type = type;
    osc.frequency.setValueAtTime(freq, t);
    g.gain.setValueAtTime(gain, t);
    g.gain.exponentialRampToValueAtTime(0.001, t + dur);
    osc.start(t);
    osc.stop(t + dur + 0.02);
  }

  return {
    place()  { tone(220, 'sine', 0.28, 0.14); tone(170, 'triangle', 0.12, 0.11); },
    move()   { tone(260, 'sine', 0.22, 0.11); },
    remove() { tone(140, 'triangle', 0.32, 0.18); tone(100, 'sine', 0.14, 0.14, 0.05); },
    mill()   {
      tone(523, 'sine', 0.18, 0.12);
      tone(659, 'sine', 0.18, 0.12, 0.11);
      tone(784, 'sine', 0.22, 0.22, 0.22);
    },
    win() {
      tone(523,  'sine', 0.16, 0.14);
      tone(659,  'sine', 0.16, 0.14, 0.15);
      tone(784,  'sine', 0.16, 0.14, 0.30);
      tone(1047, 'sine', 0.22, 0.5,  0.45);
    },
    lose() {
      tone(392, 'sine', 0.16, 0.2);
      tone(349, 'sine', 0.16, 0.2, 0.22);
      tone(294, 'sine', 0.20, 0.4, 0.44);
    },
  };
}

// ─── Board data (mirrors internal/game/board.go) ─────────────────────────────

// SVG positions matching board.go SVGPositions (600×600 viewBox)
const BOARD_POSITIONS = [
  [50,50],[300,50],[550,50],    // 0,1,2
  [550,300],[550,550],[300,550],[50,550],[50,300],  // 3,4,5,6,7
  [150,150],[300,150],[450,150],[450,300],[450,450],[300,450],[150,450],[150,300], // 8–15
  [250,250],[300,250],[350,250],[350,300],[350,350],[300,350],[250,350],[250,300], // 16–23
];

// All 16 mills
const MILLS = [
  [0,1,2],[2,3,4],[4,5,6],[6,7,0],
  [8,9,10],[10,11,12],[12,13,14],[14,15,8],
  [16,17,18],[18,19,20],[20,21,22],[22,23,16],
  [1,9,17],[3,11,19],[5,13,21],[7,15,23],
];

// ─── Alpine component ─────────────────────────────────────────────────────────

function muehleGame() {
  const _el = document.getElementById('muehle-root');
  const roomID      = _el.dataset.roomId       || '';
  const myName      = _el.dataset.playerName   || '';
  const token       = _el.dataset.token        || '';
  const spectator   = _el.dataset.spectator    === 'true';
  const aiDifficulty = _el.dataset.aiDifficulty || '';

  return {
    // state
    roomID,
    myName,
    spectator,
    aiDifficulty,
    myPlayer: null,     // 1 or 2
    phase: 'connecting', // connecting | waiting | playing | over | abandoned
    gs: null,           // GameState from server
    playerNames: ['', ''],
    selected: null,     // position index of selected own stone (move phase)
    statusMsg: '',
    gameOverStats: [],
    myWon: false,
    opponentGone: false,
    rematchRequested: false, // this player clicked Rematch
    rematchOffered: false,   // opponent wants a rematch
    muted: false,
    _sounds: null,
    startedAt: 0,   // Unix seconds; 0 = not started
    elapsedSec: 0,
    shareLink: '',
    copied: false,
    ws: null,
    _timerId: null,

    // board constants (used by isValidTarget / adjacent logic)
    positions: BOARD_POSITIONS,

    // ── lifecycle ──────────────────────────────────────────────────────────────

    init() {
      this.shareLink = window.location.href;
      this.muted = localStorage.getItem('muehle_muted') === '1';
      this._sounds = mkSounds(() => this.muted);
      // Token is pre-assigned server-side and passed directly – no async join needed.
      this.connect(token);
    },

    connect(token) {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      let url;
      if (this.spectator) {
        url = `${proto}://${location.host}/ws/${this.roomID}/spectate`;
      } else if (this.aiDifficulty) {
        url = `${proto}://${location.host}/ai/ws?difficulty=${this.aiDifficulty}`;
      } else {
        url = `${proto}://${location.host}/ws/${this.roomID}?token=${token}`;
      }
      this.ws = new WebSocket(url);

      this.ws.onopen = () => { /* connected */ };

      this.ws.onmessage = (evt) => {
        const msg = JSON.parse(evt.data);
        this.handleServerMessage(msg);
      };

      this.ws.onerror = () => {
        this.statusMsg = 'Verbindungsfehler';
      };

      this.ws.onclose = () => {
        if (this.phase !== 'over' && this.phase !== 'abandoned') {
          this.phase = 'abandoned';
        }
      };
    },

    // ── server message handling ────────────────────────────────────────────────

    handleServerMessage(msg) {
      switch (msg.type) {
        case 'waiting':
          this.phase = 'waiting';
          break;

        case 'game_start':
          this.myPlayer = msg.payload.yourPlayer;
          if (this.myPlayer === 0) {
            // spectator: server sends both names directly
            this.playerNames = [msg.payload.player1 || '?', msg.payload.player2 || '?'];
          } else {
            const opponentName = msg.payload.opponent;
            if (this.myPlayer === 1) {
              this.playerNames = [this.myName, opponentName];
            } else {
              this.playerNames = [opponentName, this.myName];
            }
          }
          this.phase = 'playing';
          this.rematchRequested = false;
          this.rematchOffered = false;
          this.selected = null;
          this.gs = null;
          if (msg.payload.startedAt) {
            this.startedAt = msg.payload.startedAt;
            this._startTimer();
          }
          this.updateStatus();
          break;

        case 'state_update':
          this.applyState(msg.payload);
          break;

        case 'game_over':
          this.myWon = msg.payload.winner === this.myPlayer;
          this.gameOverStats = msg.payload.stats || [];
          this.phase = 'over';
          this._stopTimer();
          if (this._sounds) setTimeout(() => this._sounds[this.myWon ? 'win' : 'lose'](), 200);
          break;

        case 'error':
          this.statusMsg = msg.payload?.message || 'Ungültiger Zug';
          setTimeout(() => this.updateStatus(), 2000);
          break;

        case 'opponent_left':
          this.phase = 'abandoned';
          this._stopTimer();
          break;

        case 'opponent_disconnected':
          this.opponentGone = true;
          break;

        case 'opponent_reconnected':
          this.opponentGone = false;
          break;

        case 'rematch_offer':
          this.rematchOffered = true;
          break;
      }
    },

    applyState(state) {
      if (!state) return;
      state.OnBoard1 = state.Board.filter(v => v === 1).length;
      state.OnBoard2 = state.Board.filter(v => v === 2).length;

      if (this.gs && this._sounds) {
        const prev = this.gs;
        const diff = [];
        for (let i = 0; i < 24; i++) {
          if (prev.Board[i] !== state.Board[i]) diff.push(i);
        }
        if (diff.length === 1) {
          if (prev.Board[diff[0]] === 0) this._sounds.place();
          else                           this._sounds.remove();
        } else if (diff.length === 2) {
          this._sounds.move();
        }
        if (!prev.MustRemove && state.MustRemove) {
          setTimeout(() => this._sounds && this._sounds.mill(), 170);
        }
      }

      this.gs = state;
      this.selected = null;
      this.updateStatus();
    },

    // ── status message ─────────────────────────────────────────────────────────

    updateStatus() {
      if (!this.gs || !this.myPlayer) return;
      const gs = this.gs;
      const myTurn = gs.Turn === this.myPlayer;

      if (gs.MustRemove) {
        this.statusMsg = myTurn
          ? 'Mühle! Wähle einen gegnerischen Stein zum Entfernen.'
          : `${this.playerNames[gs.Turn - 1]} schließt eine Mühle …`;
        return;
      }
      if (!myTurn) {
        this.statusMsg = `${this.playerNames[gs.Turn - 1]} ist am Zug …`;
        return;
      }
      if (gs.Phase === 0) { // PhasePlace
        this.statusMsg = `Dein Zug – setze einen Stein (${gs.ToPlace[this.myPlayer - 1]} übrig)`;
      } else {
        const flying = gs.Board.filter(v => v === this.myPlayer).length === 3;
        this.statusMsg = flying
          ? 'Dein Zug – wähle einen Stein zum Fliegen'
          : 'Dein Zug – wähle einen Stein zum Ziehen';
      }
    },

    // ── click handling ─────────────────────────────────────────────────────────

    handlePositionClick(pos) {
      if (this.spectator) return;
      if (!this.gs || this.phase !== 'playing') return;
      const gs = this.gs;
      if (gs.Turn !== this.myPlayer) return;

      // Remove phase: player must remove an opponent stone
      if (gs.MustRemove) {
        if (gs.Board[pos] !== 0 && gs.Board[pos] !== this.myPlayer) {
          this.send({ type: 'remove', pos });
        }
        return;
      }

      const myTurn = gs.Turn === this.myPlayer;
      if (!myTurn) return;

      // Place phase
      if (gs.Phase === 0) {
        if (gs.Board[pos] === 0) {
          this.send({ type: 'place', pos });
        }
        return;
      }

      // Move / fly phase
      if (this.selected === null) {
        // Select own stone
        if (gs.Board[pos] === this.myPlayer) {
          this.selected = pos;
        }
      } else {
        if (pos === this.selected) {
          // Deselect
          this.selected = null;
        } else if (gs.Board[pos] === this.myPlayer) {
          // Select a different own stone
          this.selected = pos;
        } else if (gs.Board[pos] === 0) {
          // Move to empty square
          this.send({ type: 'move', from: this.selected, to: pos });
        }
      }
    },

    // ── valid target highlighting ──────────────────────────────────────────────

    isValidTarget(pos) {
      if (this.spectator) return false;
      if (!this.gs || this.phase !== 'playing') return false;
      const gs = this.gs;
      if (gs.Turn !== this.myPlayer) return false;

      if (gs.MustRemove) {
        // Highlight removable opponent stones
        const opponent = 3 - this.myPlayer;
        if (gs.Board[pos] !== opponent) return false;
        // If stone is in a mill, only valid if all opponent stones are in mills
        if (this.isMillStone(pos)) {
          const allInMills = gs.Board
            .map((v, i) => v === opponent ? this.isMillStone(i) : true)
            .every(Boolean);
          return allInMills;
        }
        return true;
      }

      if (gs.Phase === 0) {
        return gs.Board[pos] === 0;
      }

      if (this.selected !== null) {
        if (gs.Board[pos] !== 0) return false;
        const flying = gs.Board.filter(v => v === this.myPlayer).length === 3;
        if (flying) return true;
        // Adjacent check
        const adj = this.adjacent(this.selected);
        return adj.includes(pos);
      }

      return false;
    },

    isSelected(pos) {
      return this.selected === pos;
    },

    isMillStone(pos) {
      if (!this.gs) return false;
      const board = this.gs.Board;
      const player = board[pos];
      if (player === 0) return false;
      return MILLS.some(mill =>
        mill.includes(pos) && mill.every(p => board[p] === player)
      );
    },

    adjacent(pos) {
      const adj = [
        [1,7],[0,2,9],[1,3],[2,4,11],[3,5],[4,6,13],[5,7],[6,0,15],
        [9,15],[1,8,10,17],[9,11],[3,10,12,19],[11,13],[5,12,14,21],[13,15],[7,14,8,23],
        [17,23],[16,18,9],[17,19],[18,20,11],[19,21],[20,22,13],[21,23],[22,16,15],
      ];
      return adj[pos] || [];
    },

    // ── stone appearance ───────────────────────────────────────────────────────

    stoneGradient(player) {
      if (player === 1) {
        return 'url(#stoneWhite)';
      }
      return 'url(#stoneBlack)';
    },

    // ── helpers ────────────────────────────────────────────────────────────────

    _startTimer() {
      this._stopTimer();
      this.elapsedSec = Math.floor(Date.now() / 1000) - this.startedAt;
      this._timerId = setInterval(() => {
        this.elapsedSec = Math.floor(Date.now() / 1000) - this.startedAt;
      }, 1000);
    },

    _stopTimer() {
      if (this._timerId !== null) {
        clearInterval(this._timerId);
        this._timerId = null;
      }
    },

    get timeLabel() {
      const m = Math.floor(this.elapsedSec / 60);
      const s = this.elapsedSec % 60;
      return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
    },

    send(msg) {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(msg));
      }
    },

    async copyLink() {
      await navigator.clipboard.writeText(this.shareLink);
      this.copied = true;
      setTimeout(() => this.copied = false, 2000);
    },

    requestRematch() {
      if (this.rematchRequested) return;
      this.rematchRequested = true;
      this.send({ type: 'rematch' });
    },

    toggleMute() {
      this.muted = !this.muted;
      localStorage.setItem('muehle_muted', this.muted ? '1' : '0');
    },

    get phaseLabel() {
      if (!this.gs) return '';
      const labels = ['Setzphase', 'Zugphase', 'Spielende'];
      return labels[this.gs.Phase] || '';
    },
  };
}
