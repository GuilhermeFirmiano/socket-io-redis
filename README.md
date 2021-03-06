# socket-io-redis

By running go-socket.io with this adapter, you can run multiple socket.io 
instances in different processes or servers that can all broadcast and emit 
events to and from each other.

## API

### Redis(opts map[string]string)

The following options are allowed:

- `host`: host to connect to redis on (`"localhost"`)
- `port`: port to connect to redis on (`"6379"`)
- `prefix`: the prefix of the key to pub/sub events on (`"socket.io"`)

## References

Code and README based off of:
- https://github.com/Automattic/socket.io-redis

