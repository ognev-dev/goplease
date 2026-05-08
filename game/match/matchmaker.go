package match

import (
	"log"
	"sync"
	"time"

	"github.com/ognev-dev/goplease/app/ds"
	"github.com/ognev-dev/goplease/game"
	"github.com/ognev-dev/goplease/game/bot"
	"github.com/ognev-dev/goplease/game/unit"
)

const matchmakingTimeout = 1 * time.Second

// MatchCallback is called on the searching player's goroutine when a room is ready.
type MatchCallback func(room *game.Room, playerIndex int)

type queueEntry struct {
	playerID ds.ID
	cb       MatchCallback
	at       time.Time
}

// Matchmaker pairs players or creates a bot opponent after a timeout.
type Matchmaker struct {
	mu    sync.Mutex
	queue []queueEntry
	rooms map[string]*game.Room // roomID → room

	botAI *bot.Bot
}

func New() *Matchmaker {
	mm := &Matchmaker{
		rooms: make(map[string]*game.Room),
		botAI: bot.New(),
	}
	go mm.watchQueue()
	return mm
}

// Enqueue adds a player to the matchmaking queue.
func (mm *Matchmaker) Enqueue(playerID ds.ID, cb MatchCallback) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Deduplicate — in case the client reconnects and calls new_game again.
	for _, e := range mm.queue {
		if e.playerID == playerID {
			return
		}
	}

	// If there's already someone waiting, pair them immediately.
	if len(mm.queue) > 0 {
		opponent := mm.queue[0]
		mm.queue = mm.queue[1:]

		room := mm.createRoom(opponent.playerID, playerID, false)

		log.Printf("[match] paired %s vs %s in room %s", opponent.playerID, playerID, room.ID)

		// Notify both players (callbacks may send WebSocket messages).
		go opponent.cb(room, 0)
		go cb(room, 1)
		return
	}

	mm.queue = append(mm.queue, queueEntry{
		playerID: playerID,
		cb:       cb,
		at:       time.Now(),
	})

	log.Printf("[match] player %s queued (%d in queue)", playerID, len(mm.queue))
}

// Cancel removes a player from the queue (e.g. they disconnected).
func (mm *Matchmaker) Cancel(playerID ds.ID) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	for i, e := range mm.queue {
		if e.playerID == playerID {
			mm.queue = append(mm.queue[:i], mm.queue[i+1:]...)
			log.Printf("[match] player %s removed from queue", playerID)
			return
		}
	}
}

// Room returns the active room with the given ID, or nil.
func (mm *Matchmaker) Room(roomID string) *game.Room {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.rooms[roomID]
}

// CloseRoom removes a finished room from the registry.
func (mm *Matchmaker) CloseRoom(roomID string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.rooms, roomID)
	log.Printf("[match] room %s closed", roomID)
}

// MaybeTriggerBot checks if the active player in a room is a bot and, if so,
// runs its turn asynchronously.
func (mm *Matchmaker) MaybeTriggerBot(room *game.Room) {
	// Peek at the active player without holding the room lock long.
	activeIdx := room.ActivePlayer
	p := room.Players[activeIdx]
	if !p.IsBot {
		return
	}
	go func() {
		// Small delay so the human client can see the "thinking" state.
		time.Sleep(800 * time.Millisecond)
		mm.botAI.TakeTurn(room, p)
	}()
}

// ─── Internal ─────────────────────────────────────────────────────────────────

// watchQueue periodically checks for players who've been waiting too long and
// pairs them with a bot.
func (mm *Matchmaker) watchQueue() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		mm.promoteStaleEntries()
	}
}

func (mm *Matchmaker) promoteStaleEntries() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	now := time.Now()
	remaining := mm.queue[:0]
	for _, e := range mm.queue {
		if now.Sub(e.at) >= matchmakingTimeout {
			room := mm.createRoom(e.playerID, ds.NewID(), true)
			log.Printf("[match] timeout — pairing %s with bot in room %s", e.playerID, room.ID)
			go e.cb(room, 0)

			// Immediately trigger the bot's first response if it goes second.
			go mm.botAI.TakeTurn(room, room.Players[1])
		} else {
			remaining = append(remaining, e)
		}
	}
	mm.queue = remaining
}

func (mm *Matchmaker) createRoom(p1ID, p2ID ds.ID, p2IsBot bool) *game.Room {
	deck1 := unit.StartingUnits(p1ID)
	deck2 := unit.StartingUnits(p2ID)

	p1 := game.NewPlayer(p1ID, "Player 1", 0, false, deck1)
	p2 := game.NewPlayer(p2ID, nameForPlayer(p2IsBot), 1, p2IsBot, deck2)

	room := game.NewRoom(p1, p2)
	mm.rooms[room.ID] = room
	return room
}

func nameForPlayer(isBot bool) string {
	if isBot {
		return "Bot"
	}

	return "Player 2"
}
