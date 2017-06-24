package clock

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))
)

//_Counter 支持并发的计数器
type _Counter struct {
	sync.Mutex
	counter int
}

func (counter *_Counter) AddOne() {
	counter.Lock()
	counter.counter++
	counter.Unlock()
}
func (counter *_Counter) Count() int {
	return counter.counter
}

func TestClock_Create(t *testing.T) {
	myClock := Default().Reset()
	if myClock.WaitJobs() != 0 || myClock.Count() != 0 {
		t.Errorf("JobList init have error.len=%d,count=%d", myClock.WaitJobs(), myClock.Count())
		//joblist.Debug()
	}

}

func TestClock_AddOnceJob(t *testing.T) {
	var (
		randscope = 50 * 1000 * 1000 //随机范围
		interval  = time.Millisecond*100 + time.Duration(r.Intn(randscope))
		myClock   = Default().Reset()
		jobFunc   = func() {
			//fmt.Println("任务事件")
		}
	)

	//插入时间间隔≤0，应该不允许添加
	if _, inserted := myClock.AddJobWithInterval(0, jobFunc); inserted {
		t.Error("任务添加失败，加入了间隔时间≤0的任务。")
	}

	if _, inserted := myClock.AddJobWithInterval(interval, jobFunc); !inserted {
		t.Error("任务添加失败，未加入任务。")
	}

	time.Sleep(time.Second)

	if myClock.Count() != 1 {
		t.Errorf("任务执行存在问题，应该执行%d次,实际执行%d次", 1, myClock.Count())
	}
}

//TestClock_WaitJobs 测试当前待执行任务列表中的事件
func TestClock_WaitJobs(t *testing.T) {
	var (
		myClock   = Default().Reset()
		randscope = 50 * 1000 * 1000 //随机范围
		interval  = time.Millisecond*50 + time.Duration(r.Intn(randscope))
		jobFunc   = func() {
			//fmt.Println("任务事件")
		}
	)
	job, inserted := myClock.AddJobRepeat(interval, 0, jobFunc)
	if !inserted {
		t.Error("定时任务创建失败")
	}
	time.Sleep(time.Second)

	if myClock.WaitJobs() != 1 {
		t.Error("任务添加异常")
	}
	if myClock.WaitJobs() != 1 {
		t.Error("数据列表操作获取的数据与Clock实际情况不一致！")
	}
	myClock.DelJob(job)

}

//TestClock_AddRepeatJob 测试重复任务定时执行情况
func TestClock_AddRepeatJob(t *testing.T) {
	var (
		myClock   = Default().Reset()
		jobsNum   = 1000                                                    //执行次数
		randscope = 50 * 1000                                               //随机范围
		interval  = time.Microsecond*100 + time.Duration(r.Intn(randscope)) //100-150µs时间间隔
		counter   = new(_Counter)
	)
	f := func() {
		counter.AddOne()
	}
	job, inserted := myClock.AddJobRepeat(interval, uint64(jobsNum), f)
	if !inserted {
		t.Error("任务初始化失败，任务事件没有添加成功")
	}
	for range job.C() {

	}
	//重复任务的方法是协程调用，可能还没有执行，job.C就已经退出，需要阻塞观察
	time.Sleep(time.Second)
	if int(myClock.Count()) != jobsNum || counter.Count() != jobsNum {
		t.Errorf("任务添加存在问题,应该%v次，实际执行%v\n", myClock.count, counter.Count())
	}

}

//TestClock_AddRepeatJob2 测试间隔时间不同的两个重复任务，是否会交错执行
func TestClock_AddRepeatJob2(t *testing.T) {
	var (
		myClock    = Default().Reset()
		interval1  = time.Millisecond * 20 //间隔20毫秒
		interval2  = time.Millisecond * 20 //间隔20毫秒
		singalChan = make(chan int, 10)
	)
	jobFunc := func(sigal int) {
		singalChan <- sigal

	}
	go func() {
		cacheSigal := 2
		for z := range singalChan {
			if z == cacheSigal {
				t.Error("两个任务没有间隔执行")
			} else {
				cacheSigal = z
			}
		}
	}()
	event1, inserted1 := myClock.AddJobRepeat(interval1, 0, func() { jobFunc(1) })
	time.Sleep(time.Millisecond * 10)
	event2, inserted2 := myClock.AddJobRepeat(interval2, 0, func() { jobFunc(2) })

	if !inserted1 || !inserted2 {
		t.Error("任务初始化失败，没有添加成功")
	}
	time.Sleep(time.Second)

	myClock.DelJob(event1)
	myClock.DelJob(event2)

}

//TestClock_AddMixJob 测试一次性任务+重复性任务的运行撤销情况
func TestClock_AddMixJob(t *testing.T) {
	var (
		myClock  = Default().Reset()
		counter1 int
		counter2 int
	)
	f1 := func() {
		counter1++
	}
	f2 := func() {
		counter2++
	}
	_, inserted1 := myClock.AddJobWithInterval(time.Millisecond*500, f1)
	_, inserted2 := myClock.AddJobRepeat(time.Millisecond*300, 0, f2)

	if !inserted1 && !inserted2 {
		t.Fatal("任务添加失败！")
	}
	time.Sleep(time.Second * 2)
	if counter1 != 1 || counter2 < 5 {
		t.Errorf("执行次数异常！,一次性任务执行了:%v，重复性任务执行了%v\n", counter1, counter2)
	}
}

//TestClock_AddJobs 测试短时间，高频率的情况下，事件提醒功能能否实现。
func TestClock_AddJobs(t *testing.T) {
	var (
		jobsNum   = 200000                 //添加任务数量
		randscope = 1 * 1000 * 1000 * 1000 //随机范围1秒
		myClock   = Default().Reset()
		counter   = &_Counter{}
		wg        sync.WaitGroup
	)
	f := func() {
		//schedule nothing
	}
	//创建jobsNum个任务，每个任务都会间隔[1,2)秒内执行一次
	for i := 0; i < jobsNum; i++ {
		job, inserted := myClock.AddJobWithInterval(time.Second+time.Duration(r.Intn(randscope)), f)
		if !inserted {
			t.Error("任务添加存在问题")
			break
		}
		wg.Add(1)
		go func() {
			<-job.C()
			counter.AddOne() //收到消息就计数
			wg.Done()
		}()
	}
	wg.Wait()
	if jobsNum != int(myClock.Count()) || jobsNum != counter.Count() {
		t.Errorf("应该执行%v次，实际执行%v次,外部信号接受到%v次。\n", jobsNum, myClock.Count(), counter.Count())
	}
}

//TestClock_DelJob 检测待运行任务中，能否随机删除一条任务。
func TestClock_DelJob(t *testing.T) {
	//思路：
	//新增一定数量的任务，延时1秒开始执行
	//在一秒内，删除所有的任务。
	//如果执行次数=0，说明一秒内无法满足对应条数的增删
	var (
		jobsNum   = 20000
		randscope = 1 * 1000 * 1000 * 1000
		jobs      = make([]Job, jobsNum)
		delmod    = r.Intn(jobsNum)
		myClock   = Default().Reset()
	)
	for i := 0; i < jobsNum; i++ {
		delay := time.Second + time.Duration(r.Intn(randscope)) //增加一秒作为延迟，以避免删除的时候，已经存在任务被通知执行，导致后续判断失误
		job, _ := myClock.AddJobWithInterval(delay, nil)
		jobs[i] = job
	}

	deleted := myClock.DelJob(jobs[delmod])
	if !deleted || myClock.WaitJobs() != uint(jobsNum-1) {
		t.Errorf("任务删除%v，删除后，应该只剩下%v条任务，实际还有%v条\n", deleted, myClock.Count(), jobsNum-1)

	}
}

//TestClock_DelJobs 本测试主要检测添加、删除任务的性能。保证每秒1万次新增+删除操作。
func TestClock_DelJobs(t *testing.T) {
	//思路：
	//新增一定数量的任务，延时1秒开始执行
	//在一秒内，删除所有的任务。
	//如果执行次数！=0，说明一秒内无法满足对应条数的增删
	var (
		myClock     = NewClock().Reset()
		jobsNum     = 20000
		randscope   = 1 * 1000 * 1000 * 1000
		jobs        = make([]Job, jobsNum)
		wantdeljobs = make([]Job, jobsNum)
	)
	for i := 0; i < jobsNum; i++ {
		delay := time.Second + time.Duration(r.Intn(randscope)) //增加一秒作为延迟，以避免删除的时候，已经存在任务被通知执行，导致后续判断失误
		job, _ := myClock.AddJobWithInterval(delay, nil)
		jobs[i] = job
		wantdeljobs[i] = job
	}

	myClock.DelJobs(wantdeljobs)

	if 0 != int(myClock.Count()) || myClock.WaitJobs() != 0 {
		t.Errorf("应该执行%v次，实际执行%v次,此时任务队列中残余记录,myClock.actionindex.len=%v,\n", jobsNum-len(wantdeljobs), myClock.Count(), myClock.WaitJobs())

	}
}

//TestClock_Delay_200kJob 测试2秒内能否执行20万条任务。
// Note:笔记本(尤其是windows操作系统）,云服务可能无法通过测试
func TestClock_Delay_200kJob(t *testing.T) {
	// skip just for pass travis because of lack of performance
	t.Skip()
	var (
		jobsNum     = 200000 //添加任务数量
		myClock     = NewClock()
		jobInterval = time.Second
	)
	fn := func() {
		//do nothing
	}

	//初始化20万条任务。考虑到初始化耗时，延时1秒后启动
	for i := 0; i < jobsNum; i++ {
		myClock.AddJobWithInterval(jobInterval, fn)

	}

	time.Sleep(time.Second * 3)
	if jobsNum != int(myClock.Count()) {
		t.Errorf("应该执行%v次，实际执行%v次。\n", jobsNum, myClock.Count())
	}
}

func TestClock_Stop(t *testing.T) {
	var (
		jobsNum     = 1000
		myClock     = NewClock()
		jobInterval = time.Millisecond * 100
		count       = int32(0)
	)
	fn := func() {
		atomic.AddInt32(&count, 1)
	}
	for i := 0; i < jobsNum; i++ {
		myClock.AddJobWithInterval(jobInterval*time.Duration(i), fn)
	}

	myClock.Stop()
	time.Sleep(time.Second * 1)
	if count > 0 {
		t.Errorf("定时器没有正常结束，执行了%d次，实际应该为0.", count)
	}
}

func TestClock_StopGracefull(t *testing.T) {
	var (
		jobsNum     = 2000
		myClock     = NewClock()
		jobInterval = time.Millisecond * 100
		count       = int32(0)
	)
	fn := func() {
		atomic.AddInt32(&count, 1)
	}
	for i := 0; i < jobsNum; i++ {
		myClock.AddJobRepeat(time.Second+jobInterval*time.Duration(i), 1, fn)
	}
	myClock.StopGracefull()
	if count != int32(jobsNum) {
		t.Errorf("定时器没有正常结束，执行了%d次，实际应该为%v\n.", count, jobsNum)
	}
}
func BenchmarkClock_AddJob(b *testing.B) {
	myClock := NewClock().Reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, inserted := myClock.AddJobWithInterval(time.Second*5, nil)
		if !inserted {
			b.Error("can not insert jobItem")
			break
		}
	}
}
