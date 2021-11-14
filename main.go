package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
	"io"
	"log"
	"net"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

func main() {

	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	var dnstapfile = flag.String("file", "", "read dnstap data from file")
	var idFlag = flag.Bool("id", false, "include DNS ID in output")

	flag.Parse()

	if *cpuprofile != "" {
		pf, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(pf)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	if *dnstapfile == "" {
		flag.Usage()
		os.Exit(1)
	}

	f, err := os.Open(*dnstapfile)
	if err != nil {
		log.Fatal(err)
	}

	opts := &dnstap.ReaderOptions{}

	r, err := dnstap.NewReader(f, opts)
	if err != nil {
		log.Fatal(err)
	}

	dec := dnstap.NewDecoder(r, 8192)

	timeFormat := "02-Jan-2006 15:04:05.000"

	// Use buffered output to speed things up
	bufStdout := bufio.NewWriter(os.Stdout)

	for {

		var dt dnstap.Dnstap

		if err := dec.Decode(&dt); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		var err error
		var t time.Time
		var sb strings.Builder
		var queryAddress, responseAddress string

		qa := net.IP(dt.Message.QueryAddress)
		ra := net.IP(dt.Message.ResponseAddress)

		msg := new(dns.Msg)

		isQuery := strings.HasSuffix(dnstap.Message_Type_name[int32(*dt.Message.Type)], "_QUERY")

		// Query address: 10.10.10.10:31337 or ?
		if qa != nil {
			queryAddress = qa.String() + ":" + strconv.FormatUint(uint64(*dt.Message.QueryPort), 10)
		} else {
			queryAddress = "?"
		}

		// Response address: 10.10.10.10:31337 or ?
		if ra != nil {
			responseAddress = ra.String() + ":" + strconv.FormatUint(uint64(*dt.Message.ResponsePort), 10)
		} else {
			responseAddress = "?"
		}

		if isQuery {
			err = msg.Unpack(dt.Message.QueryMessage)
			if err != nil {
				log.Printf("unable to unpack query message (%s -> %s): %s", queryAddress, responseAddress, err)
				msg = nil
			}
			t = time.Unix(int64(*dt.Message.QueryTimeSec), int64(*dt.Message.QueryTimeNsec))
		} else {
			err = msg.Unpack(dt.Message.ResponseMessage)
			if err != nil {
				log.Printf("unable to unpack response message (%s <- %s): %s", queryAddress, responseAddress, err)
				msg = nil
			}
			t = time.Unix(int64(*dt.Message.ResponseTimeSec), int64(*dt.Message.ResponseTimeNsec))
		}

		// Timestamp, like 27-Oct-2021 18:29:47.412
		sb.WriteString(t.Local().Format(timeFormat))

		switch *dt.Message.Type {
		case dnstap.Message_AUTH_QUERY:
			sb.WriteString(" AQ ")
		case dnstap.Message_AUTH_RESPONSE:
			sb.WriteString(" AR ")
		case dnstap.Message_CLIENT_QUERY:
			sb.WriteString(" CQ ")
		case dnstap.Message_CLIENT_RESPONSE:
			sb.WriteString(" CR ")
		case dnstap.Message_FORWARDER_QUERY:
			sb.WriteString(" FQ ")
		case dnstap.Message_FORWARDER_RESPONSE:
			sb.WriteString(" FR ")
		case dnstap.Message_RESOLVER_QUERY:
			sb.WriteString(" RQ ")
		case dnstap.Message_RESOLVER_RESPONSE:
			sb.WriteString(" RR ")
		case dnstap.Message_STUB_QUERY:
			sb.WriteString(" SQ ")
		case dnstap.Message_STUB_RESPONSE:
			sb.WriteString(" SR ")
		case dnstap.Message_TOOL_QUERY:
			sb.WriteString(" TQ ")
		case dnstap.Message_TOOL_RESPONSE:
			sb.WriteString(" TR ")
		default:
			log.Fatalf("Unexpected message type: %s", *dt.Message.Type)
		}

		// Query address
		sb.WriteString(queryAddress)

		// Direction arrow
		if isQuery {
			sb.WriteString(" -> ")
		} else {
			sb.WriteString(" <- ")
		}

		// Response address
		sb.WriteString(responseAddress)

		// UDP or TCP
		sb.WriteString(" ")
		sb.WriteString(dt.Message.SocketProtocol.String())

		// Size of message, like "37b"
		sb.WriteString(" ")
		if isQuery {
			sb.WriteString(strconv.Itoa(len(dt.Message.QueryMessage)))
		} else {
			sb.WriteString(strconv.Itoa(len(dt.Message.ResponseMessage)))
		}
		sb.WriteString("b")

		// Record: name.example.com/IN/A or ?/?/?
		sb.WriteString(" ")
		// For cases where we were unable to unpack the DNS message we
		// return ?/?/?. The same goes for an empty question section.
		if msg == nil || len(msg.Question) == 0 {
			sb.WriteString("?/?/?")
		} else {
			// The name is printed without the trailing dot unless it specifically is the root zone
			if msg.Question[0].Name == "." {
				sb.WriteString(msg.Question[0].Name)
			} else {
				sb.WriteString(msg.Question[0].Name[:len(msg.Question[0].Name)-1])
			}
			sb.WriteString("/")
			// IN, CH etc or synthesized "CLASS31337" based on the
			// numeric value if not a known class
			if c, ok := dns.ClassToString[msg.Question[0].Qclass]; ok {
				sb.WriteString(c)
			} else {
				sb.WriteString("CLASS")
				sb.WriteString(strconv.FormatUint(uint64(msg.Question[0].Qclass), 10))
			}
			sb.WriteString("/")
			// A, MX, NS etc or synthesized "TYPE31337" based on the
			// numeric value if not a known type
			if t, ok := dns.TypeToString[msg.Question[0].Qtype]; ok {
				sb.WriteString(t)
			} else {
				sb.WriteString("TYPE")
				sb.WriteString(strconv.FormatUint(uint64(msg.Question[0].Qtype), 10))
			}

			if *idFlag {
				// ID: 31337
				sb.WriteString(" ID: ")
				sb.WriteString(strconv.FormatUint(uint64(msg.Id), 10))
			}
		}

		// One message per line
		sb.WriteString("\n")

		fmt.Fprint(bufStdout, sb.String())
	}

	err = bufStdout.Flush()
	if err != nil {
		log.Fatal(err)
	}
}
