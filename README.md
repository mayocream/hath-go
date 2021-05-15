# Hath-Go

> Hentai@Home (H@H) is an open-source Peer-2-Peer gallery distribution system which reduces the load on the E-Hentai Galleries.

(unofficial) Go port of H@H p2p server.

## Compare to official JAVA program

**Advantages**:
- Easy to run (No JIT environment and external steps)
- Higher performance (based on [fasthttp](https://github.com/valyala/fasthttp))

**Disadvantages**:
- Unstable (you might loss trust for unexpected shutdown)
- No GUI provided

## Install

### From Source

```bash
$ go install github.com/mayocream/hath-go/cli/hath
```

## Usage

Edit your client config:
```bash
$ mkdir ~/.hath
$ nano ~/.hath/config.yaml
```

Copy/Paste these example config to `~/.hath/config.yaml`
```yaml
client_id: ""
client_key: ""
db_file: ""
```

```bash
$ hath -f config.yaml
```

## Development/Test

Change config file, print debug logs: 
```yaml
debug: true
log_level: debug
```


## Todolist

- [] Goreleaser
- [] Fully test
- [] Documentation
- [] Support HTTP/3 (QUIC)
- [] GUI
