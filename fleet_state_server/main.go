package main

import "fmt"
import "net"
import "os"
import "flag"
import "time"
import "strings"
import "strconv"
import "math"

var helptext = `Usage: fleet_state_server

  A fleet state server listens for location updates from individual vehicles.
  It sends a stream of update packets to clients who have subscribed to
  updates about a specific vehicle.

Options:
  --host <string>           IP address the server will listen on.
                            Default: "localhost".
  --port <int>              Port number the server will listen on.
                            Default: 8000.

Flags:
  -h, --help                Print this help text and exit.
  --verbose                 Print a log of all incoming packets.
`

// This command line flag turns on verbose output -- if set to true the server will print a log of
// all incoming packets. I'm cheating here and using a global variable to avoid cluttering up the
// code by passing the value around all the functions. In production code, we might want to pass
// this value as an argument.
var verbose bool

// We use this type to store location updates from individual vehicles in the fleet. For each
// vehicle, we store a list of all previous [location] updates.
type location struct {
	timestamp time.Time
	latitude  float64
	longitude float64
}

func main() {
	// This is the IP address the server will listen on.
	var host string
	flag.StringVar(&host, "host", "localhost", "IP address for server.")

	// This is the port number the server will listen on.
	var port string
	flag.StringVar(&port, "port", "8000", "Port number for server.")

	// If set to true, we print a log of all incoming packets.
	flag.BoolVar(&verbose, "verbose", false, "Turn on verbose output.")

	flag.Usage = func() {
		fmt.Print(helptext)
	}

	flag.Parse()
	runServer(host, port)
}

func runServer(host string, port string) {
	serverAddr, err := net.ResolveUDPAddr("udp", host+":"+port)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error: unable to resolve server address '%s:%s'.\n  -->  %s\n",
			host,
			port,
			err.Error())
		os.Exit(1)
	}

	listener, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error: unable to initialize listener on address '%s:%s'.\n  -->  %s\n",
			host,
			port,
			err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("--------------------------")
	fmt.Println("Running Fleet State Server")
	fmt.Println("--------------------------")
	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Port: %s\n", port)
	fmt.Printf("Exit: Ctrl-C\n")
	fmt.Println("--------------------------")

	// This is the server's primary data store. Each key is a VIN string. Each value is a list of
	// timestamped [location] structs for that vehicle.
	fleet := make(map[string][]location)

	// This is the server's subscriber store. Each key is a VIN string. Each value is a list of
	// subscriber client addresses for that VIN.
	subscribers := make(map[string][]*net.UDPAddr)

	// This is the server loop -- it will continue to listen for incoming UDP packets until the
	// user terminates the server with Ctrl-C.
	for {
		buffer := make([]byte, 256)

		n, addr, err := listener.ReadFromUDP(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid read.\n.  -->  %s\n", err.Error())
			continue
		}

		handlePacket(addr, string(buffer[:n]), fleet, subscribers)
	}
}

// This function handles incoming UDP packets. It assumes that packets are either SUBSCRIBE requests
// from clients or update packets from vehicles.
func handlePacket(source *net.UDPAddr, message string, fleet map[string][]location, subscribers map[string][]*net.UDPAddr) {
	if verbose {
		fmt.Println(source, ">>", message)
	}

	if strings.HasPrefix(message, "SUBSCRIBE") {
		handleSubscriberPacket(source, message, subscribers)
	} else {
		handleVehiclePacket(message, fleet, subscribers)
	}
}

// This function handles incoming SUBSCRIBE packets from clients. A SUBSCRIBE request packet is
// assumed to have the format: [SUBSCRIBE <vin>]. The subscriber's address is added to the list
// of subscribers for that VIN.
func handleSubscriberPacket(source *net.UDPAddr, message string, subscribers map[string][]*net.UDPAddr) {
	elements := strings.Split(message, " ")
	if len(elements) != 2 {
		fmt.Fprintf(os.Stderr, "Error: invalid subscriber packet.\n")
		return
	}

	vin := elements[1]

	if _, ok := subscribers[vin]; ok {
		subscribers[vin] = append(subscribers[vin], source)
	} else {
		subscribers[vin] = []*net.UDPAddr{source}
	}
}

// This function handles incoming update packets from vehicles. An update packet is assumed to have
// the format: [<timestamp> <vin> <latitude> <longitude>].
func handleVehiclePacket(message string, fleet map[string][]location, subscribers map[string][]*net.UDPAddr) {
	elements := strings.Split(message, " ")
	if len(elements) != 4 {
		fmt.Fprintf(os.Stderr, "Error: invalid vehicle packet.\n")
		return
	}

	timestamp, err := time.Parse(time.RFC3339Nano, elements[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid timestamp.\n")
		return
	}

	vin := elements[1]

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

	entry := location{timestamp: timestamp, latitude: latitude, longitude: longitude}

	if _, ok := fleet[vin]; ok {
		fleet[vin] = append(fleet[vin], entry)
	} else {
		fleet[vin] = []location{entry}
	}

	// If one or more clients have subscribed to updates about this particular vehicle, send
	// each of them an update packet.
	if subscriberList, ok := subscribers[vin]; ok {
		sendSubscriberUpdate(subscriberList, fleet[vin], vin)
	}
}

// This function sends an update packet to each subscriber in the subscribers list.
func sendSubscriberUpdate(subscribers []*net.UDPAddr, locations []location, vin string) {
	// The vehicle's speed in meters per second. A value of -1.0 means we don't have enough
	// information to calculate the speed.
	speed := -1.0

	// We need at least two locations to try calculating the speed.
	length := len(locations)
	if length > 1 {
		loc1 := locations[length-2]
		loc2 := locations[length-1]

		// The duration in seconds between last two location timestamps.
		duration := loc2.timestamp.Sub(loc1.timestamp).Seconds()

		// Only try calculating the speed if the gap between the timestamps is less than 2 seconds.
		if duration < 2.0 {
			distance := getDistance(loc1.latitude, loc1.longitude, loc2.latitude, loc2.longitude)
			speed = distance / duration
		}
	}

	lastLocation := locations[length-1]
	timestamp := lastLocation.timestamp.Format(time.RFC3339Nano)
	latitude := lastLocation.latitude
	longitude := lastLocation.longitude
	message := fmt.Sprintf("%s %s %.6f %.6f %.6f", timestamp, vin, latitude, longitude, speed)

	for _, addr := range subscribers {
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Error: unable to connect to subscriber address '%s'.\n  -->  %s\n",
				addr,
				err.Error())
			return
		}

		_, err = conn.Write([]byte(message))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to send subscriber update.\n  -->  %s\n", err.Error())
		}

		conn.Close()
	}
}

// This function returns the great-circle distance in meters between two points on the earth's
// surface calculated using the haversine formula. This formula remains well-conditioned for small
// distances with an error of up to approx 0.5%. Latitude and longitude are assumed to be specified
// in degrees.
// Ref: http://www.movable-type.co.uk/scripts/latlong.html
func getDistance(lat1, long1, lat2, long2 float64) float64 {
	// Average radius of the earth in meters.
	const earthRadius = 6371009

	phi1 := lat1 * math.Pi / 180.0     // latitude 1 in radians
	lambda1 := long1 * math.Pi / 180.0 // longitude 1 in radians

	phi2 := lat2 * math.Pi / 180.0     // latitude 2 in radians
	lambda2 := long2 * math.Pi / 180.0 // longitude 2 in radians

	deltaPhi := phi2 - phi1
	deltaLambda := lambda2 - lambda1

	a := math.Pow(math.Sin(deltaPhi/2), 2) + math.Cos(phi1)*math.Cos(phi2)*math.Pow(math.Sin(deltaLambda/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
