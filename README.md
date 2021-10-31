# dnstap-parse

The main goal of this code is to create a basic [dnstap](https://dnstap.info) printing tool based on
the [golang-dnstap](https://github.com/dnstap/golang-dnstap) library.

The output is supposed to mimic the "short summary format" of
[dnstap-read](https://github.com/isc-projects/bind9/blob/main/bin/tools/dnstap-read.c)
from BIND but with the possibility of adding additional information via flags
so you can easily grep for such things (currently DNS ID via `-id`)

```
Usage of ./dnstap-parse:
  -cpuprofile string
    	write cpu profile to file
  -file string
    	read dnstap data from file
  -id
    	include DNS ID in output
```

The `-cpuprofile` flag is not helpful for ordinary usage, it is just there to
be able to [profile](https://go.dev/blog/pprof) the tool.

## Known output differences with dnstap-read

From investigating dnstap files in the wild I have noticed some instances
where the output of this tool and dnstap-read differs. Specifically the
character escaping rules used by dnstap-read and miekg/dns differ somewhat.

One example of this is how `0x20` (space) is represented in domain names, where miekg/dns
will present it as `\ ` and dnstap-read will present it as `\032` leading to
this tool outputting `example\ lookup/IN/A` while dnstap-read will print
`example\032lookup/IN/A`.

Another example of this is the `0x27` (`'`) character which is not escaped at
all by dnstap-read, but is escpaed in miekg/dns due to being defined as special
in [isDomainNameLabelSpecial()](https://github.com/miekg/dns/blob/3b8982ccc6a0de0e195b964bcdd57da6fe119cbe/types.go#L595)

This results in dnstap-read outputting `example'lookup/IN/A` while this
tools prints `example\'lookup/IN/A`.

The overall character espacing rules used by miekg/dns can be found in
[UnpackDomainName()](https://github.com/miekg/dns/blob/3b8982ccc6a0de0e195b964bcdd57da6fe119cbe/msg.go#L373)
