moxy
====

A Golang MQTT proxy providing useful output traces to monitor and troubleshoot your MQTT communications.

## Installing moxy

```
go get github.com/jvermillard/moxy
```

## Using moxy

```
./bin/moxy
```

By default you will now have a local MQTT proxy to iot.eclipse.org's broker!

To use a different broker, or expose your local proxy on e.g a different port, checkout the `-help` flag

```
./bin/moxy -help
```
