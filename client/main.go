package main

import "fmt"
import "net"
import "os"
import "flag"
import "time"
import "strings"
import "strconv"

var helptext = `Usage: client

  A client subscribes to a feed of updates about a specific vehicle. The client
  will continue listening for updates until the user terminates the process by
  hitting Ctrl-C.

Options:
  --client-host <string>    IP address that the client will listen on.
                            Default: "localhost".
  --client-port <int>       Port number that the client will listen on.
                            Default: 8001.
  --server-host <string>    IP address of the fleet server.
                            Default: "localhost"
  --server-port <int>       Port number of the fleet server.
                            Default: 8000.
  --vin <string>            VIN of the target vehicle to subscribe to.
                            Default: "1HGBH41JXMN000000".

Flags:
  -h, --help                Print this help text and exit.
`

func main() {
    // This is the IP address the client will listen on for updates.
    var localHost string
    flag.StringVar(&localHost, "client-host", "localhost", "IP address for client.")

    // This is the port number the client will listen on for updates.
    // Improvement: figure out how to automatically select an available port number to use as the
    // default to make it easier to run multiple clients simultaneously.
    var localPort string
    flag.StringVar(&localPort, "client-port", "8001", "Port number for client.")

    // This is the IP address of the server the client will subscribe to.
    var remoteHost string
    flag.StringVar(&remoteHost, "server-host", "localhost", "IP address for server.")

    // This is the port number of the server the client will subscribe to.
    var remotePort string
    flag.StringVar(&remotePort, "server-port", "8000", "Port number for server.")

    // This is the VIN of the target vehicle.
    var vin string
    flag.StringVar(&vin, "vin", "1HGBH41JXMN000000", "VIN of target vehicle.")

    flag.Usage = func() {
        fmt.Print(helptext)
    }

    flag.Parse()

    // This is the local address of the client. The client will send its subscription request from
    // this address and it will listen on this address for updates from the server.
    localAddr, err := net.ResolveUDPAddr("udp", localHost + ":" + localPort)
    if err != nil {
        fmt.Fprintf(
            os.Stderr,
            "Error: unable to resolve client address '%s:%s'.\n  -->  %s\n",
            localHost,
            localPort,
            err.Error())
        os.Exit(1)
    }

    // This is the remote address of the fleet server. The client will send its subscription
    // request to this address.
    remoteAddr, err := net.ResolveUDPAddr("udp", remoteHost + ":" + remotePort)
    if err != nil {
        fmt.Fprintf(
            os.Stderr,
            "Error: unable to resolve server address '%s:%s'.\n  -->  %s\n",
            remoteHost,
            remotePort,
            err.Error())
        os.Exit(1)
    }

    runClient(localAddr, remoteAddr, vin)
}

// The client sends a subscription request packet to the fleet state server, then listens for
// incoming update packets from the server.
func runClient(localAddr *net.UDPAddr, remoteAddr *net.UDPAddr, vin string) {
    fmt.Println("-------------------------");
    fmt.Println("Running Subscriber Client")
    fmt.Println("-------------------------");
    fmt.Printf("Client: %s\n", localAddr)
    fmt.Printf("Server: %s\n", remoteAddr)
    fmt.Printf("VIN:    %s\n", vin)
    fmt.Printf("Exit:   Ctrl-C\n")
    fmt.Println("-------------------------");

    // Send a SUBSCRIBE packet to the server.
    message := fmt.Sprintf("SUBSCRIBE %s", vin)

    // This will fail if the local port is already being used by another client.
    conn, err := net.DialUDP("udp", localAddr, remoteAddr)
    if err != nil {
        fmt.Fprintf(
            os.Stderr,
            "Error: unable to connect to '%s' from '%s'.\n  -->  %s\n",
            remoteAddr,
            localAddr,
            err.Error())
        os.Exit(1)
    }

    _, err = conn.Write([]byte(message))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: failed to send subscription packet.\n  -->  %s\n", err.Error())
        os.Exit(1)
    }

    conn.Close()

    // Listen for incoming update packets.
    listener, err := net.ListenUDP("udp", localAddr)
    if err != nil {
        fmt.Fprintf(
            os.Stderr,
            "Error: unable to initialize listener on address '%s'.\n  -->  %s\n",
            localAddr,
            err.Error())
        os.Exit(1)
    }
    defer listener.Close()

    // This is the client's listening loop. It will continue listening for update packets until the
    // user hits Ctrl-C.
    for {
        buffer := make([]byte, 256)

        n, _, err := listener.ReadFromUDP(buffer)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: invalid read.\n.  -->  %s\n", err.Error())
            continue
        }

        handlePacket(string(buffer[:n]))
    }
}

// An update packet should have the format: [<timestamp> <vin> <latitude> <longitude> <speed>].
func handlePacket(message string) {
    elements := strings.Split(message, " ")
    if len(elements) != 5 {
        fmt.Fprintf(os.Stderr, "Error: invalid update packet.\n")
        return
    }

    timestamp, err := time.Parse(time.RFC3339Nano, elements[0])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: invalid timestamp.\n")
        return
    }

    latitude, err := strconv.ParseFloat(elements[2], 64)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: invalid latitude.\n")
        return
    }

    longitude, err := strconv.ParseFloat(elements[3], 64)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: invalid longitude.\n")
        return
    }

    speed, err := strconv.ParseFloat(elements[4], 64)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: invalid speed.\n")
        return
    }

    timeString := timestamp.Format(time.RFC3339)

    // A speed value of -1.0 means the speed is not available.
    if speed == -1.0 {
        fmt.Printf("[%s]  (%.6f, %.6f)  N/A\n", timeString, latitude, longitude)
    } else {
        fmt.Printf("[%s]  (%.6f, %.6f)  %5.2f m/s\n", timeString, latitude, longitude, speed)
    }
}
