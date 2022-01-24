# Fleet Simulator

This repository contains three command-line binaries, all written in Go.

* **vehicle_simulator** &mdash; Simulates a fleet of independent vehicles.
* **fleet_state_server** &mdash; Listens for location updates from simulated vehicles.
* **client** &mdash; Subscribes to a feed of updates about a specific vehicle.

To build the binaries, clone the repository, `cd` into the `fleetsim` directory and run `make`:

    $ git clone https://github.com/dmulholl/fleetsim.git
    $ cd fleetsim
    $ make

The binaries will be placed in a new `fleetsim/bin/` directory.

The binaries have no dependencies outside of the Go standard library. They should build cleanly without any need to set `$GOPATH`, etc.

NB &mdash; I'm assuming that the binaries will be built and run on a *unixy* system. I've tested them on Mac and Linux but not on Windows. All testing has been with Go version 1.17.6.



## Overview

All communication happens via UDP packets.

* Each simulated vehicle sends a location-update packet to the fleet server once per second.
  This packet contains a timestamp, the vehicle's VIN, and its latitude and longitude coordinates.

* The server listens for incoming update packets from individual vehicles.
  It stores a list of timestamped locations for each vehicle in the fleet.

* The server also listens for incoming subscription requests from clients.
  A client can subscribe to a feed of location and speed updates for a particular vehicle by
  specifying its VIN.

* The server sends an update packet to the client every time it receives a location update from the
  vehicle the client has subscribed to.

* Multiple clients can run simultaneously and multiple clients can subscribe to update feeds for
  the same vehicle.

* By default, all communication happens over localhost, although in theory the three binaries could
  be run on three separate machines &mdash; you'd just need to specify the IP addresses on the
  command line. (I haven't actually tested this though!)

Each binary is intended to be run in its own terminal window as they print their output to stdout.

You can shut down a binary by hitting `Ctrl-C`. In general the binaries can be started and stopped in
any order. (Do note that the client currently sends a single subscription request so if the client
runs before the server its request packet will be lost in the ether.)



## The Fleet State Server

The server monitors the state of the fleet, listening for incoming location packets from vehicles
and sending updates to clients.

    Usage: fleet_state_server

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

The server defaults to listening on port `8000`. You may need to specify a different port number if this
port is already in use on your machine.

Limitation &mdash; once a client has subscribed to a stream of updates, the server sends an endless
stream of update packets in its direction. It should really listen for a periodic 'keep-alive'
packet and terminate the subscription after a fixed timeout has elapsed if it hasn't heard from
the client.



## The Vehicle Simulator

The simulator runs each simulated vehicle in its own goroutine.

    Usage: vehicle_simulator

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

The simulator prints the VIN of each simulated vehicle. You can use these VINs to subscribe clients
to feeds for specific vehicles.

Limitation &mdash; the simulated vehicles aren't very realistic but they do produce the right *kind* of
data!



## The Client

A client subscribes to a feed of updates about a specific vehicle.

    Usage: client

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

Use the `--vin <string>` option to specify the target vehicle.
If omitted, it defaults to the vehicle with the VIN `1HGBH41JXMN000000`, which is always the first
vehicle launched by the simulator.

If you want to run multiple clients simultaneously you'll need to use the `--client-port <int>`
option to specify a unique port number for each one to listen on.

Limitation &mdash; the client currently sends a single subscription request packet to the server.
If this gets lost, the client will never receive any updates. It should really send subscription
requests in a loop until it gets a response.
