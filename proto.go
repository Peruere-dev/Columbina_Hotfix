package main



type ProtoWriter struct {
	buf []byte
}

func NewProtoWriter() *ProtoWriter {
	return &ProtoWriter{}
}

func (w *ProtoWriter) Bytes() []byte {
	return w.buf
}

func putVarint(buf []byte, v uint64) int {
	i := 0
	for v >= 0x80 {
		buf[i] = byte(v) | 0x80
		v >>= 7
		i++
	}
	buf[i] = byte(v)
	return i + 1
}

func varintLen(v uint64) int {
	i := 0
	for v >= 0x80 {
		i++
		v >>= 7
	}
	return i + 1
}

func (w *ProtoWriter) writeTag(field int, wireType int) {
	v := uint64(field<<3) | uint64(wireType)
	var buf [10]byte
	n := putVarint(buf[:], v)
	w.buf = append(w.buf, buf[:n]...)
}

func (w *ProtoWriter) WriteVarint(field int, v uint64) {
	w.writeTag(field, 0)
	var buf [10]byte
	n := putVarint(buf[:], v)
	w.buf = append(w.buf, buf[:n]...)
}

func (w *ProtoWriter) WriteBool(field int, v bool) {
	if v {
		w.WriteVarint(field, 1)
	} else {
		w.WriteVarint(field, 0)
	}
}

func (w *ProtoWriter) WriteInt32(field int, v int32) {
	w.WriteVarint(field, uint64(uint32(v)))
}

func (w *ProtoWriter) WriteString(field int, v string) {
	if v == "" {
		return
	}
	w.writeTag(field, 2)
	var buf [10]byte
	n := putVarint(buf[:], uint64(len(v)))
	w.buf = append(w.buf, buf[:n]...)
	w.buf = append(w.buf, v...)
}

func (w *ProtoWriter) WriteBytes(field int, v []byte) {
	if v == nil {
		return
	}
	w.writeTag(field, 2)
	var buf [10]byte
	n := putVarint(buf[:], uint64(len(v)))
	w.buf = append(w.buf, buf[:n]...)
	w.buf = append(w.buf, v...)
}

func BuildRegionSimpleInfo(name, title, type_, dispatchURL string) []byte {
	w := NewProtoWriter()
	w.WriteString(1, name)
	w.WriteString(2, title)
	w.WriteString(3, type_)
	w.WriteString(4, dispatchURL)
	return w.Bytes()
}

func BuildResVersionConfig(version uint32, relogin bool, md5 string, releaseTotalSize string, versionSuffix, branch string, nextScriptVersion uint32) []byte {
	w := NewProtoWriter()
	w.WriteVarint(1, uint64(version))
	w.WriteBool(2, relogin)
	w.WriteString(3, md5)
	w.WriteString(4, releaseTotalSize)
	w.WriteString(5, versionSuffix)
	w.WriteString(6, branch)
	if nextScriptVersion != 0 {
		w.WriteVarint(7, uint64(nextScriptVersion))
	}
	return w.Bytes()
}

type RegionInfoParams struct {
	GateserverIP               string
	GateserverPort             uint32
	AreaType                   string
	ResourceURL                string
	DataURL                    string
	ResourceURLBak             string
	DataURLBak                 string
	ClientDataVersion          uint32
	ClientSilenceDataVersion   uint32
	ClientDataMD5              string
	ClientSilenceDataMD5       string
	ResVersionConfig           []byte
	ClientVersionSuffix        string
	ClientSilenceVersionSuffix string
}

func BuildRegionInfo(p RegionInfoParams) []byte {
	w := NewProtoWriter()
	w.WriteString(1, p.GateserverIP)
	w.WriteVarint(2, uint64(p.GateserverPort))
	w.WriteString(7, p.AreaType)
	w.WriteString(8, p.ResourceURL)
	w.WriteString(9, p.DataURL)
	w.WriteString(12, p.ResourceURLBak)
	w.WriteString(13, p.DataURLBak)
	w.WriteVarint(14, uint64(p.ClientDataVersion))
	w.WriteVarint(18, uint64(p.ClientSilenceDataVersion))
	w.WriteString(19, p.ClientDataMD5)
	w.WriteString(20, p.ClientSilenceDataMD5)
	w.WriteBytes(22, p.ResVersionConfig)
	w.WriteString(26, p.ClientVersionSuffix)
	w.WriteString(27, p.ClientSilenceVersionSuffix)
	return w.Bytes()
}

func BuildStopServerInfo(stopBeginTime, stopEndTime uint32, url, contentMsg string) []byte {
	w := NewProtoWriter()
	w.WriteVarint(1, uint64(stopBeginTime))
	w.WriteVarint(2, uint64(stopEndTime))
	w.WriteString(3, url)
	w.WriteString(4, contentMsg)
	return w.Bytes()
}

func BuildForceUpdateInfo(forceUpdateURL string) []byte {
	w := NewProtoWriter()
	if forceUpdateURL != "" {
		w.WriteString(1, forceUpdateURL)
	}
	if w.buf == nil {
		return []byte{}
	}
	return w.Bytes()
}

type QueryCurRegionRsp struct {
	Retcode                       int32
	Msg                           string
	RegionInfo                    []byte
	ForceUpdate                   []byte
	StopServer                    []byte
	ClientSecretKey               []byte
	RegionCustomConfigEncrypted   []byte
	ClientRegionCustomConfigEncrypted []byte
	ConnectGateTicket             string
}

func BuildQueryCurRegionRsp(rsp QueryCurRegionRsp) []byte {
	w := NewProtoWriter()
	w.WriteInt32(1, rsp.Retcode)
	w.WriteString(2, rsp.Msg)
	w.WriteBytes(3, rsp.RegionInfo)
	w.WriteBytes(4, rsp.ForceUpdate)
	w.WriteBytes(5, rsp.StopServer)
	w.WriteBytes(11, rsp.ClientSecretKey)
	w.WriteBytes(12, rsp.RegionCustomConfigEncrypted)
	w.WriteBytes(13, rsp.ClientRegionCustomConfigEncrypted)
	w.WriteString(14, rsp.ConnectGateTicket)
	return w.Bytes()
}

type QueryRegionListRsp struct {
	Retcode                    int32
	RegionList                 [][]byte
	ClientSecretKey            []byte
	ClientCustomConfigEncrypted []byte
	EnableLoginPC              bool
}

func BuildQueryRegionListRsp(rsp QueryRegionListRsp) []byte {
	w := NewProtoWriter()
	w.WriteInt32(1, rsp.Retcode)
	for _, r := range rsp.RegionList {
		w.WriteBytes(2, r)
	}
	w.WriteBytes(5, rsp.ClientSecretKey)
	w.WriteBytes(6, rsp.ClientCustomConfigEncrypted)
	w.WriteBool(7, rsp.EnableLoginPC)
	return w.Bytes()
}


