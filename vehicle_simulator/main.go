package main

import "fmt"
import "net"
import "os"
import "time"
import "os/signal"
import "flag"
import "math"
import "math/rand"

var helptext = `Usage: vehicle_simulator

  This binary simulates a fleet of independent vehicles. Each vehicle in the
  fleet sends a location update once per second to the fleet state server.

Options:
  --host <string>           IP address of the fleet state server.
                            Default: "localhost".
  --number <int>            Number of vehicles in the simulated fleet.
                            Default: 20.
  --port <int>              Port number of the fleet state server.
                            Default: 8000.

Flags:
  -h, --help                Print this help text and exit.
`

func main() {
    var host string
    flag.StringVar(&host, "host", "localhost", "IP address for server.")

    var port string
    flag.StringVar(&port, "port", "8000", "Port number for server.")

    var number int
    flag.IntVar(&number, "number", 20, "Number of vehicles.")

    flag.Usage = func() {
        fmt.Print(helptext)
    }

    flag.Parse()
    rand.Seed(time.Now().UnixNano())
    runSimulator(host, port, number)
}

func runSimulator(host string, port string, numVehicles int) {
    serverAddr, err := net.ResolveUDPAddr("udp", host + ":" + port)
    if err != nil {
        fmt.Fprintf(
            os.Stderr,
            "Error: unable to resolve server address '%s:%s'.\n  -->  %s\n",
            host,
            port,
            err.Error())
        os.Exit(1)
    }

    fmt.Println("-------------------------")
    fmt.Println("Running Vehicle Simulator")
    fmt.Println("-------------------------")
    fmt.Printf("Num Vehicles: %d\n", numVehicles)
    fmt.Printf("Server Host:  %s\n", host)
    fmt.Printf("Server Port:  %s\n", port)
    fmt.Printf("Exit:         Ctrl-C\n")
    fmt.Println("-------------------------")

    // Launch a goroutine for each simulated vehicle in the fleet.
    for i := 0; i < numVehicles; i++ {
        go simulateVehicle(serverAddr, i)
    }

    // Block until the user hits Ctrl-C.
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    _ = <-c

    fmt.Println("\n-------------------------")
    fmt.Println("Shutting down....")
    fmt.Println("-------------------------")
}

// This function simulates a single vehicle, sending location update packets to the fleet state
// server once per second. It's not a very realistic simulation but it generates the right *kind*
// of data.
func simulateVehicle(serverAddr *net.UDPAddr, serialNumber int) {
    vin := makeVIN(serialNumber)
    fmt.Println("VIN:", vin)

    // The vehicle's initial position. Every vehicle starts off in the centre of Dublin at the front
    // gate of Trinity College. Working with latitude/longitude coordinates to six decimal places
    // gives us accuracy to within about 11cm.
    // Ref: https://en.wikipedia.org/wiki/Decimal_degrees
    latitude := 53.344496
    longitude := -6.259427

    // The vehicle's initial speed in meters per second -- 100 km/h is approximately 28 m/s.
    // We select a random speed in the range [0, 28.0).
    speed := rand.Float64() * 28.0

    // The vehicles initial direction. This is an angle in radians randomly selected from the
    // interval [0, 2 * pi). The angle is measured anticlockwise from the reference direction
    // which is due east.
    direction := rand.Float64() * 2 * math.Pi

    for {
        speed = updateSpeed(speed)
        latitude, longitude = updateLocation(latitude, longitude, speed, direction, 1.0)
        timestamp := time.Now().UTC().Format(time.RFC3339Nano)
        message := fmt.Sprintf("%s %s %.6f %.6f", timestamp, vin, latitude, longitude)

        conn, err := net.DialUDP("udp", nil, serverAddr)
        if err != nil {
            fmt.Fprintf(
                os.Stderr,
                "Error: unable to connect to server '%s'.\n  -->  %s\n",
                serverAddr,
                err.Error())
            continue
        }

        _, err = conn.Write([]byte(message))
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: failed to send packet.\n  -->  %s\n", err.Error())
        }

        conn.Close()
        time.Sleep(time.Second)
    }
}

// This function returns a valid-ish VIN. The template is a random VIN I grabbed from the internet.
func makeVIN(number int) string {
    return fmt.Sprintf("1HGBH41JXMN%06d", number)
}

// This function randomly varies the vehicle's speed, assuming a maximum acceleration of 5 m/s/s.
// It always returns a value in the range [0, 28].
func updateSpeed(speed float64) float64 {
    // Select a random delta in the range [-5, 5).
    delta := rand.Float64() * 10 - 5
    speed += delta

    if speed < 0 {
        return 0
    } else if speed > 28.0 {
        return 28.0
    }

    return speed
}

// This function calculates the vehicle's new latitude and longitude coordinates. This is a fairly
// rough approximation that should be fine for generating some sample data, but for real accuracy
// we'd need to use proper spherical geometry.
//
// Assumptions:
// - latitude and longitude are measured in degrees
// - speed is measured in meters per second
// - direction is an angle measured in radians
// - duration is measured in seconds
func updateLocation(latitude, longitude, speed, direction, duration float64) (float64, float64) {
    // The straight-line distance in meters traveled by the vehicle.
    distance := speed * duration

    // East-West delta in meters -- we use this to update the longitude.
    deltaX := distance * math.Cos(direction)

    // North-South delta in meters -- we use this to update the latitude.
    deltaY := distance * math.Sin(direction)

    // The conversion between surface distance in meters and degrees of latitude is independent of
    // location. 1 degree of latitude is equal to 111,111 meters so 1 meter is equal to about
    // 0.000_009 degrees of latitude.
    // Ref: https://gis.stackexchange.com/questions/8650/measuring-accuracy-of-latitude-and-longitude
    newLatitude := latitude + deltaY * 0.000009

    // The conversion between surface distance in meters and degrees of longitude depends on the
    // latitude: 1 degree longitude = 111_319.5 * cos(latitude) meters.
    // Ref: https://en.wikipedia.org/wiki/Decimal_degrees
    degreesOfLongitudePerMeter := 1 / (111319.5 * math.Cos(latitude * math.Pi / 180.0))
    newLongitude := longitude + deltaX * degreesOfLongitudePerMeter

    return newLatitude, newLongitude
}
