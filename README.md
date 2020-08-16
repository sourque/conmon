# conmon

Portable connection monitor (conmon) for system administration written in Go.

## Screenshots



## Features

- Live feed of connections to the system
- Kill (working on blocking/examining/etc) connections
- Fun and quickly usable UI


idealand
- track bandwidth used per process, data sent, capture data and output to file or terminal
    - get proc fds and start listenin' (duplicate fd and redirect? other, better method?)
- catch fast connections: inotify in proc and check if binding? refresh faster? delay tcp handshake? conntrack table?
- c copy to clipboard

client-server conns
- connect to remote sessions and combine things, able to send commands over ssl
