package settlement

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Scheduler handles scheduled settlement tasks
type Scheduler struct {
	service    *Service
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	running    bool
	mu         sync.Mutex
	lastRun    time.Time
	lastResult *BatchSettlementResult
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	// Interval between settlement checks (default: 1 hour)
	CheckInterval time.Duration
	// Day of week to run settlements (0 = Sunday, 1 = Monday, etc.)
	SettlementDay time.Weekday
	// Hour of day to run settlements (0-23, UTC)
	SettlementHour int
}

// DefaultSchedulerConfig returns the default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		CheckInterval:  1 * time.Hour,
		SettlementDay:  time.Sunday, // Run on Sundays
		SettlementHour: 0,           // At midnight UTC
	}
}

// NewScheduler creates a new settlement scheduler
func NewScheduler(service *Service, config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	return &Scheduler{
		service:  service,
		interval: config.CheckInterval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the scheduled settlement processing
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)

	log.Println("Settlement scheduler started")
	return nil
}

// Stop stops the scheduled settlement processing
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	log.Println("Settlement scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetLastRun returns the time of the last settlement run
func (s *Scheduler) GetLastRun() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRun
}

// GetLastResult returns the result of the last settlement run
func (s *Scheduler) GetLastResult() *BatchSettlementResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastResult
}


// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start if it's settlement time
	s.checkAndRunSettlement(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRunSettlement(ctx)
		}
	}
}

// checkAndRunSettlement checks if it's time to run settlement and executes if needed
func (s *Scheduler) checkAndRunSettlement(ctx context.Context) {
	// Get the previous settlement period
	periodStart, periodEnd := s.service.GetPreviousSettlementPeriod()

	// Check if we've already processed this period
	s.mu.Lock()
	if !s.lastRun.IsZero() && s.lastRun.After(periodEnd) {
		s.mu.Unlock()
		return // Already processed this period
	}
	s.mu.Unlock()

	// Run the settlement batch
	log.Printf("Running settlement for period %s to %s", periodStart.Format(time.RFC3339), periodEnd.Format(time.RFC3339))
	
	result, err := s.service.ProcessSettlementBatch(ctx, periodStart, periodEnd)
	if err != nil {
		log.Printf("Settlement batch failed: %v", err)
		return
	}

	// Update last run info
	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastResult = result
	s.mu.Unlock()

	log.Printf("Settlement completed: %d creators processed, %d successful, %d failed, total amount: $%s",
		result.TotalCreators, result.SuccessCount, result.FailedCount, result.TotalAmount.String())
}

// RunNow triggers an immediate settlement run for the previous period
func (s *Scheduler) RunNow(ctx context.Context) (*BatchSettlementResult, error) {
	periodStart, periodEnd := s.service.GetPreviousSettlementPeriod()
	
	result, err := s.service.ProcessSettlementBatch(ctx, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	// Update last run info
	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastResult = result
	s.mu.Unlock()

	return result, nil
}

// RunForPeriod triggers a settlement run for a specific period
func (s *Scheduler) RunForPeriod(ctx context.Context, periodStart, periodEnd time.Time) (*BatchSettlementResult, error) {
	result, err := s.service.ProcessSettlementBatch(ctx, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	// Update last run info
	s.mu.Lock()
	s.lastRun = time.Now()
	s.lastResult = result
	s.mu.Unlock()

	return result, nil
}

// SchedulerStatus represents the current status of the scheduler
type SchedulerStatus struct {
	Running           bool                   `json:"running"`
	LastRun           *time.Time             `json:"last_run,omitempty"`
	LastResult        *BatchSettlementResult `json:"last_result,omitempty"`
	NextScheduledRun  time.Time              `json:"next_scheduled_run"`
	CurrentPeriodStart time.Time             `json:"current_period_start"`
	CurrentPeriodEnd   time.Time             `json:"current_period_end"`
}

// GetStatus returns the current status of the scheduler
func (s *Scheduler) GetStatus() *SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentStart, currentEnd := s.service.GetCurrentSettlementPeriod()
	
	status := &SchedulerStatus{
		Running:            s.running,
		CurrentPeriodStart: currentStart,
		CurrentPeriodEnd:   currentEnd,
		NextScheduledRun:   currentEnd, // Settlement runs at the end of each period
	}

	if !s.lastRun.IsZero() {
		status.LastRun = &s.lastRun
	}
	if s.lastResult != nil {
		status.LastResult = s.lastResult
	}

	return status
}
