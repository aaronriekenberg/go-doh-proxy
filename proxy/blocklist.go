package proxy

import (
	"bufio"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/miekg/dns"
)

func installHandlersForBlockedDomains(blockedDomainsFile string, serveMux *dns.ServeMux, blockedDomainHandler dns.HandlerFunc) {
	log.Printf("reading BlockedDomainsFile %q", blockedDomainsFile)
	file, err := os.Open(blockedDomainsFile)
	if err != nil {
		log.Fatalf("error reading BlockedDomainsFile: %v", err)
	}
	defer file.Close()

	var blockedDomainsSlice []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		blockedDomain := strings.TrimSpace(scanner.Text())
		if len(blockedDomain) == 0 {
			continue
		}
		blockedDomain = dns.CanonicalName(blockedDomain)
		blockedDomainsSlice = append(blockedDomainsSlice, blockedDomain)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("BlockedDomainsFile scanner error: %v", err)
	}

	sort.Slice(blockedDomainsSlice, func(i, j int) bool {
		return len(blockedDomainsSlice[i]) < len(blockedDomainsSlice[j])
	})

	alreadyBlocked := make(map[string]bool)

	// would like to use dns.ServeMux.match but it is not exported
	domainIsAlreadyBlocked := func(domainName string) bool {
		for off, end := 0, false; !end; off, end = dns.NextLabel(domainName, off) {
			if blocked := alreadyBlocked[domainName[off:]]; blocked {
				return true
			}
		}
		return false
	}

	skippedBlockedDomain := 0
	handlersInstalled := 0
	for _, blockedDomain := range blockedDomainsSlice {

		if domainIsAlreadyBlocked(blockedDomain) {
			skippedBlockedDomain++
		} else {
			serveMux.HandleFunc(blockedDomain, blockedDomainHandler)
			handlersInstalled++
			alreadyBlocked[blockedDomain] = true
		}

	}

	log.Printf("all blocked domains %v skippedBlockedDomain %v handlersInstalled %v", len(blockedDomainsSlice), skippedBlockedDomain, handlersInstalled)
}
