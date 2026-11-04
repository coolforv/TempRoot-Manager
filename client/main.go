package main
import("encoding/binary";"encoding/json";"fmt";"io";"net";"os")
type Request struct{Op string `json:"op"`;Args []string `json:"args,omitempty"`}
func main(){
 if len(os.Args)!=3||os.Args[1]!="-c"{fmt.Fprintln(os.Stderr,"usage: trm-su -c <command>");os.Exit(2)}
 c,e:=net.Dial("unix","/data/local/tmp/temproot.sock");if e!=nil{fmt.Fprintln(os.Stderr,"broker unavailable:",e);os.Exit(127)};defer c.Close()
 if e=json.NewEncoder(c).Encode(Request{Op:"shell",Args:[]string{"-c",os.Args[2]}});e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(127)}
 var h [8]byte;if _,e=io.ReadFull(c,h[:]);e!=nil{fmt.Fprintln(os.Stderr,"authorization/protocol failure");os.Exit(126)}
 n:=binary.LittleEndian.Uint32(h[:4]);code:=int32(binary.LittleEndian.Uint32(h[4:]));if n>16*1024*1024{fmt.Fprintln(os.Stderr,"invalid response size");os.Exit(126)}
 out:=make([]byte,n);if _,e=io.ReadFull(c,out);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(126)};_,_=os.Stdout.Write(out);if code<0||code>255{code=126};os.Exit(int(code))
}
