package redis

import (
	"log"
	"strings"

	"github.com/garyburd/redigo/redis"
	uuid "github.com/nu7hatch/gouuid"

	socketio "github.com/GuilhermeFirmiano/socket-io"
	"github.com/GuilhermeFirmiano/socket-io-redis/cmap_string_cmap"
	"github.com/GuilhermeFirmiano/socket-io-redis/cmap_string_socket"

	"encoding/json"
)

type broadcast struct {
	host     string
	port     string
	password string
	pub      redis.PubSubConn
	sub      redis.PubSubConn
	prefix   string
	uid      string
	key      string
	remote   bool
	rooms    cmap_string_cmap.ConcurrentMap
}

// opts: {
//   "host": "127.0.0.1",
//   "port": "6379"
//   "prefix": "socket.io"
// }
func Redis(opts map[string]string) socketio.Broadcast {
	b := broadcast{
		rooms: cmap_string_cmap.New(),
	}

	options := []redis.DialOption{}

	var ok bool
	b.host, ok = opts["host"]
	if !ok {
		b.host = "127.0.0.1"
	}
	b.port, ok = opts["port"]
	if !ok {
		b.port = "6379"
	}
	b.prefix, ok = opts["prefix"]
	if !ok {
		b.prefix = "socket.io"
	}

	b.password, ok = opts["password"]
	if ok {
		options = append(options, redis.DialPassword(b.password))
	}

	pub, err := redis.Dial("tcp", b.host+":"+b.port, options...)
	if err != nil {
		panic(err)
	}
	sub, err := redis.Dial("tcp", b.host+":"+b.port, options...)
	if err != nil {
		panic(err)
	}

	b.pub = redis.PubSubConn{Conn: pub}
	b.sub = redis.PubSubConn{Conn: sub}

	uid, err := uuid.NewV4()
	if err != nil {
		log.Println("error generating uid:", err)
		return nil
	}
	b.uid = uid.String()
	b.key = b.prefix + "#" + b.uid

	b.remote = false

	b.sub.PSubscribe(b.prefix + "#*")

	// This goroutine receives and prints pushed notifications from the server.
	// The goroutine exits when there is an error.
	go func() {
		for {
			switch n := b.sub.Receive().(type) {
			case redis.Message:
				log.Printf("Message: %s %s\n", n.Channel, n.Data)
			case redis.PMessage:
				b.onmessage(n.Channel, n.Data)
				log.Printf("PMessage: %s %s %s\n", n.Pattern, n.Channel, n.Data)
			case redis.Subscription:
				log.Printf("Subscription: %s %s %d\n", n.Kind, n.Channel, n.Count)
				if n.Count == 0 {
					return
				}
			case error:
				log.Printf("error: %v\n", n)
				return
			}
		}
	}()

	return b
}

func (b broadcast) onmessage(channel string, data []byte) error {
	pieces := strings.Split(channel, "#")
	uid := pieces[len(pieces)-1]
	if b.uid == uid {
		log.Println("ignore same uid")
		return nil
	}

	var out map[string][]interface{}
	err := json.Unmarshal(data, &out)
	if err != nil {
		log.Println("error decoding data")
		return nil
	}

	args := out["args"]
	opts := out["opts"]

	room, ok := opts[0].(string)
	if !ok {
		log.Println("room is not a string")
		room = ""
	}
	message, ok := opts[1].(string)
	if !ok {
		log.Println("message is not a string")
		message = ""
	}

	b.remote = true
	b.Send(room, message, args...)
	return nil
}

func (b broadcast) Join(room string, socket socketio.Conn) {
	sockets, ok := b.rooms.Get(room)
	if !ok {
		sockets = cmap_string_socket.New()
	}
	sockets.Set(socket.ID(), socket)
	b.rooms.Set(room, sockets)
}

func (b broadcast) Leave(room string, socket socketio.Conn) {
	sockets, ok := b.rooms.Get(room)
	if !ok {
		return
	}
	sockets.Remove(socket.ID())
	if sockets.IsEmpty() {
		b.rooms.Remove(room)
	}

	b.rooms.Set(room, sockets)
}

func (b broadcast) LeaveAll(socket socketio.Conn) {}

// Same as Broadcast
func (b broadcast) Send(room, message string, args ...interface{}) {
	sockets, ok := b.rooms.Get(room)
	if !ok {
		return
	}
	for item := range sockets.Iter() {
		s := item.Val
		s.Emit(message, args...)
	}

	opts := make([]interface{}, 3)
	opts[0] = room
	opts[1] = message
	in := map[string][]interface{}{
		"args": args,
		"opts": opts,
	}

	buf, err := json.Marshal(in)
	_ = err

	if !b.remote {
		b.pub.Conn.Do("PUBLISH", b.key, buf)
	}
	b.remote = false
}

func (b broadcast) SendAll(event string, args ...interface{}) {}

func (b broadcast) Len(room string) int {
	return len(b.rooms)
}

func (b broadcast) Clear(room string) {
	b.rooms.Remove(room)
}

// Rooms gives the list of all the rooms available for broadcast in case of
// no connection is given, in case of a connection is given, it gives
// list of all the rooms the connection is joined to
func (b broadcast) Rooms(socket socketio.Conn) []string {

	rooms := make([]string, 0)
	if socket == nil {
		for _, room := range b.rooms.GetAll() {
			for r := range room {
				rooms = append(rooms, r)
			}
		}
	} else { // create a new list of all the room names the connection is joined to
		rooms = append(rooms, socket.Rooms()...)
	}
	return rooms
}

func (b broadcast) ForEach(room string, f socketio.EachFunc) {
}
