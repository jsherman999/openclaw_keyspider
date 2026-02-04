package worker

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/spider"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

type ScanWorker struct {
	cfg   *config.Config
	db    *db.DB
	st    *store.Store
	sp    *spider.Spider
	poll  time.Duration
}

func NewScanWorker(cfg *config.Config, dbc *db.DB) *ScanWorker {
	return &ScanWorker{cfg: cfg, db: dbc, st: store.New(dbc), sp: spider.New(cfg, dbc), poll: 2 * time.Second}
}

func (w *ScanWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := w.st.ClaimNextScanJob(ctx)
		if err != nil {
			// no job or transient error
			time.Sleep(w.poll)
			continue
		}
		if job == nil {
			time.Sleep(w.poll)
			continue
		}

		since := time.Duration(job.SinceSec) * time.Second
		log.Printf("scan_worker: running job id=%d host=%s since=%s depth=%d", job.ID, job.TargetHost, since, job.SpiderDepth)

		err = w.runOne(ctx, job, since)
		if err2 := w.st.FinishScanJob(ctx, job.ID, err); err2 != nil {
			log.Printf("scan_worker: finish job id=%d error=%v", job.ID, err2)
		}
	}
}

func (w *ScanWorker) runOne(ctx context.Context, job *store.ScanJob, since time.Duration) error {
	if job.Kind != "scan" {
		return errors.New("unknown job kind")
	}
	_, err := w.sp.ScanHost(ctx, job.TargetHost, since, job.SpiderDepth)
	return err
}
