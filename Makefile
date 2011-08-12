include $(GOROOT)/src/Make.inc

TARG=lzo
CGOFILES=lzo.go

#include $(GOROOT)/src/Make.cmd
include $(GOROOT)/src/Make.pkg

lzopack:	lzopack.go lzo.go
	6g -I. lzopack.go && 6l -L. -o lzopack lzopack.6
