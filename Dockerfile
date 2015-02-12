FROM busybox

ADD dnscock /dnscock

EXPOSE 53/udp

ENTRYPOINT ["/dnscock"] 
