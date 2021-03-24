package main

import (
	"fmt"
	"sync"
)

type conf struct {
	concurrency int
	workload    int
}

func (c conf) String() string {
	return fmt.Sprintf("concurrency:%d workload:%d", c.concurrency, c.workload)
}

// 班长布置的作业
type assignJob struct {
	data int
}

// 给老师看的作业
type toReviewJob struct {
	data string
}

type idelStudent struct {
	j   *assignJob
	dst chan<- *assignJob
}
type finishedJob struct {
	j   *toReviewJob
	src chan<- *toReviewJob
}

type multiChan struct {
	conf
	wg               sync.WaitGroup
	closeCh          chan struct{}
	assignJobChans   []chan *assignJob   // 给同学布置作业的通道
	idelStudentPool  chan *idelStudent   // 空闲的同学，班长从班主任那里拿到作业后，随机分发给同学做
	giveBackJobChans []chan *toReviewJob // 等待老师检查后，通过这个通道把的作业还给同学，让他下次可以继续做作业
	toReviewJobChan  chan *finishedJob   // 同学完成任务后把结果返回给老师
}

func newMultiChan(c conf) *multiChan {
	mc := &multiChan{
		conf:    c,
		closeCh: make(chan struct{}),
	}
	mc.assignJobChans = make([]chan *assignJob, mc.concurrency)
	for i := 0; i < mc.concurrency; i++ {
		mc.assignJobChans[i] = make(chan *assignJob, 1)
	}
	mc.idelStudentPool = make(chan *idelStudent, mc.concurrency)
	for i := 0; i < mc.concurrency; i++ {
		mc.idelStudentPool <- &idelStudent{
			j:   new(assignJob),
			dst: mc.assignJobChans[i],
		}
	}
	mc.giveBackJobChans = make([]chan *toReviewJob, mc.concurrency)
	for i := 0; i < mc.concurrency; i++ {
		mc.giveBackJobChans[i] = make(chan *toReviewJob, 1)
		mc.giveBackJobChans[i] <- new(toReviewJob)
	}
	mc.toReviewJobChan = make(chan *finishedJob, mc.concurrency)
	return mc
}

func (mc *multiChan) close() {
	close(mc.closeCh)

	close(mc.idelStudentPool)
	for range mc.idelStudentPool {
	}
	for i := range mc.assignJobChans {
		close(mc.assignJobChans[i])
		for range mc.assignJobChans[i] {

		}
	}
	close(mc.toReviewJobChan)
	for i := range mc.giveBackJobChans {
		close(mc.giveBackJobChans[i])
		for range mc.giveBackJobChans[i] {

		}
	}
}

// 学生干活
func (mc *multiChan) goStudent(id int) {
	defer func() {
		fmt.Printf("%d 号同学下线\n", id)
		mc.wg.Done()
	}()
	var ok bool
	for {
		var giveBackJob *toReviewJob
		// 拿自己的作业本
		select {
		case <-mc.closeCh:
			return
		case giveBackJob, ok = <-mc.giveBackJobChans[id]:
		}
		if !ok {
			return
		}
		doneJob := &finishedJob{
			j:   giveBackJob,
			src: mc.giveBackJobChans[id], // 让老师通过这个通过送回给自己
		}
		// 拿班长布置的作业
		var job *assignJob
		select {
		case <-mc.closeCh:
			return
		case job, ok = <-mc.assignJobChans[id]:
		}
		if !ok {
			return
		}
		// 做作业
		doneJob.j.data = fmt.Sprintf("%d 完成了 %d 号作业", id, job.data)
		// 给老师
		mc.toReviewJobChan <- doneJob
		// 自己空闲了
		mc.idelStudentPool <- &idelStudent{
			j:   job,
			dst: mc.assignJobChans[id],
		}
	}
}

// 班长发作业
func (mc *multiChan) goClassMonitor() {
	defer func() {
		fmt.Println("班长发完作业了")
		mc.wg.Done()
	}()
	var ok bool
	for i := 0; i < mc.workload; i++ {
		// 给空闲同学的同学派发作业
		var stu *idelStudent
		select {
		case <-mc.closeCh:
			return
		case stu, ok = <-mc.idelStudentPool:
		}
		if !ok {
			return
		}
		stu.j.data = i
		stu.dst <- &assignJob{
			data: stu.j.data,
		}
	}
}

// 老师叫班长发作业，自己改作业
func (mc *multiChan) goTeacher() {
	go mc.goClassMonitor()
	for i := 0; i < mc.concurrency; i++ {
		go mc.goStudent(i)
	}
	mc.wg.Add(1 + mc.concurrency)
	jobCount := mc.workload
	var ok bool
	for {
		// 给同学检查作业
		var job *finishedJob
		select {
		case <-mc.closeCh:
			return
		case job, ok = <-mc.toReviewJobChan:
		}
		if !ok {
			return
		}
		// 检查作业
		fmt.Println(job.j.data)
		// 把作业给回同学
		job.src <- job.j
		// 如果作业全部做完就退出
		jobCount--
		if jobCount == 0 {
			break
		}
	}
	fmt.Println("老师改完完作业了")
	mc.close()
	mc.wg.Wait()
}

func main(){
	mc := newMultiChan(conf{
		concurrency: 4,
		workload:    200,
	})
	mc.goTeacher()
}
