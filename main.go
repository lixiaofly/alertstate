package alertstate

import (
	"fmt"
	"sync"
	"time"
)

type entryRecord struct {
	timestamp   int64
	class       classType
	sniffer     int32
	sniffername string
	site        int32
	sitename    string
}

var GLocalCach LocalCache
var InputCh = make(chan entryRecord, InputCacheLenDef)

type LocalCache struct {
	lock        *sync.Mutex
	WinWidth    int32 //秒
	WinNum      int32
	InputCh     chan entryRecord
	MaxWinId    int64
	Windows     map[int64]*window
	WillBeDelId int64
	Handler     func(*GlobalResult)
}

func (this *LocalCache) Init(width int32, num int32) (error, *LocalCache) {
	this.lock = new(sync.Mutex)
	if width <= 0 || num < 1 {
		return ErrWinParamentErr, nil
	}
	this.WinWidth = width
	this.WinNum = num
	this.InputCh = InputCh
	this.Windows = make(map[int64]*window)
	return nil, this
}

func (this *LocalCache) GetWindowTime(winid int64) int64 {
	return int64(winid) * int64(this.WinWidth)
}

func (this *LocalCache) GetOvertimeWinid() int32 {
	return int32(int64(this.MaxWinId) - int64(this.WinNum))
}

var deleteWindowCount = 0

func (this *LocalCache) deleteWindow(id int64) error {
	//move to global cache
	this.MvToGlobal(id)
	//to handler(websocket)
	//TODO
	result := gGlobalCache.ToSlice()
	this.Handler(result)
	//fmt.Println("result", result)
	return nil
}

func (this *LocalCache) MvToGlobal(id int64) error {
	gGlobalCache.lock.Lock()
	defer gGlobalCache.lock.Unlock()

	if _, ok := this.Windows[id]; !ok {
		deleteWindowCount++
		fmt.Println("delete window not exist count=", deleteWindowCount)
		return ErrWindowNotExist
	}

	gGlobalCache.time = this.Windows[id].time
	gGlobalCache.merge((this.Windows[id].mp))
	gGlobalCache.snifTypeNumRt = this.Windows[id].mp.snifTypeNum
	gGlobalCache.snifferNumRt = this.Windows[id].mp.siteNum
	//free map
	delete(this.Windows, id)
	return nil
}

func (this *LocalCache) insert(winid int64, data entryRecord) {
	//record name
	gIdNameMap.Insert(
		idNameT{data.sniffer, data.sniffername},
		idNameT{data.site, data.sitename})
	//insert
	this.Windows[winid].insert(data)
}

func (this *LocalCache) Insert2(data entryRecord) (err error) {
	winid := data.timestamp / int64(this.WinWidth)

	if this.MaxWinId == 0 {
		/*
			new window
			value maxid
			insert
		*/
		this.Windows[winid] = new(window).init(this.GetWindowTime(winid))
		this.MaxWinId = winid
	} else if winid == this.MaxWinId-1 {
		/*
			new window if not exist
			insert
		*/
		if _, ok := this.Windows[winid]; !ok {
			this.Windows[winid] = new(window).init(this.GetWindowTime(winid))
		}
	} else if winid == this.MaxWinId {
		/*
			insert
		*/
	} else if winid == this.MaxWinId+1 {
		/*
			new window
			value maxid
			go delete old window
			insert
		*/
		this.Windows[winid] = new(window).init(this.GetWindowTime(winid))
		this.deleteWindow(this.MaxWinId - 1)
		this.MaxWinId = winid
	} else if winid < this.MaxWinId-1 {
		/*
			insert global window
		*/
		gGlobalCache.Insert(data)
		return
	} else if winid > this.MaxWinId+1 {
		/*
			delete all window
			new window
			value maxid
			insert
		*/
		for wid, _ := range this.Windows {
			this.MvToGlobal(wid)
		}
		this.Windows[winid] = new(window).init(this.GetWindowTime(winid))
		this.MaxWinId = winid
	}
	this.insert(winid, data)
	return nil
}

func (this *LocalCache) Start() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			//			if this.MaxWinId != 0 {
			//				fmt.Println("cur maxid window:", this.Windows[this.MaxWinId])
			//			}
			//fmt.Println("gGlobalCache=", &gGlobalCache)
		}
	}()
	for {
		select {
		case data := <-this.InputCh:
			this.Insert2(data)
		}
	}
}

//func main() {
//	go GLocalCach.Start()
//	//startTest()
//}
