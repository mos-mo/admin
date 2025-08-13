package main

// gRPC 서버 연결 및 Overview 프레임 스트림 수신을 담당하는 App 구현
// - 앱 시작 시 서버에 자동 연결
// - Overview 구독으로 들어오는 프레임을 base64 로 인코딩 후 프론트로 이벤트 전송
// - 최신 프레임 스냅샷 조회 메서드 제공 (프론트에서 필요 시 호출)

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"admin/proto"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// gRPC 서버 주소 (추후 환경변수/설정화 가능)
	GRPC_SERVER_ADDRESS = "localhost:50051"
	// 연결 재시도 간격
	RECONNECT_INTERVAL_MS = 3000
	// 이벤트 이름 상수
	EVENT_OVERVIEW_FRAME = "overviewFrame"
)

// frameSnapshot는 최신 프레임 캐시 구조입니다.
type frameSnapshot struct {
	AgentID   string `json:"agentId"`
	ImageBase string `json:"imageBase64"`
	IsPreview bool   `json:"isPreview"`
	Timestamp int64  `json:"timestamp"`
}

// App 구조체 (Wails 바인딩)
type App struct {
	ctx          context.Context
	conn         *grpc.ClientConn
	adminClient  proto.AdminServiceClient
	cancel       context.CancelFunc
	framesMu     sync.RWMutex
	latestFrames map[string]*frameSnapshot
}

// NewApp App 생성자
func NewApp() *App {
	return &App{latestFrames: make(map[string]*frameSnapshot)}
}

// startup Wails 앱 시작 훅
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.bootstrapLoop()
}

// bootstrapLoop 서버 연결 및 재시도 루프를 수행합니다.
func (a *App) bootstrapLoop() {
	for {
		if err := a.connectAndSubscribe(); err != nil {
			log.Printf("[Admin][BOOT] 연결/구독 실패: %v", err)
			time.Sleep(RECONNECT_INTERVAL_MS * time.Millisecond)
			continue
		}
		// 정상 종료(스트림 끝) 시 재연결 시도
		log.Printf("[Admin][BOOT] 스트림 종료 - 재연결 대기")
		time.Sleep(RECONNECT_INTERVAL_MS * time.Millisecond)
	}
}

// connectAndSubscribe gRPC 연결 후 Overview 구독을 시작합니다.
func (a *App) connectAndSubscribe() error {
	// 기존 연결 정리
	if a.conn != nil {
		_ = a.conn.Close()
	}
	conn, err := grpc.Dial(GRPC_SERVER_ADDRESS, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	a.conn = conn
	a.adminClient = proto.NewAdminServiceClient(conn)
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	log.Printf("[Admin][BOOT] 서버 연결 성공: %s", GRPC_SERVER_ADDRESS)
	return a.subscribeOverview(ctx)
}

// subscribeOverview Overview 스트림을 구독하여 이벤트로 전파합니다.
func (a *App) subscribeOverview(ctx context.Context) error {
	adminID := fmt.Sprintf("admin-%d", time.Now().UnixNano())
	stream, err := a.adminClient.SubscribeOverview(ctx, &proto.AdminSubscribeRequest{AdminId: adminID})
	if err != nil {
		return fmt.Errorf("subscribe overview: %w", err)
	}
	log.Printf("[Admin][STREAM] overview 구독 시작: %s", adminID)
	for {
		frame, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		// 프레임 처리 후 이벤트 발행
		bs := base64.StdEncoding.EncodeToString(frame.GetImageData())
		a.storeFrame(frame, bs)
		runtime.EventsEmit(a.ctx, EVENT_OVERVIEW_FRAME, map[string]any{
			"agentId":     frame.GetAgentId(),
			"imageBase64": bs,
			"isPreview":   frame.GetIsPreview(),
			"timestamp":   frame.GetTimestamp(),
		})
	}
}

// storeFrame 최신 프레임을 캐시합니다.
func (a *App) storeFrame(f *proto.FrameData, base64Str string) {
	a.framesMu.Lock()
	a.latestFrames[f.GetAgentId()] = &frameSnapshot{
		AgentID:   f.GetAgentId(),
		ImageBase: base64Str,
		IsPreview: f.GetIsPreview(),
		Timestamp: f.GetTimestamp(),
	}
	a.framesMu.Unlock()
}

// GetLatestFrames 현재까지 수신한 최신 프레임 목록을 반환합니다.
func (a *App) GetLatestFrames() []frameSnapshot {
	a.framesMu.RLock()
	list := make([]frameSnapshot, 0, len(a.latestFrames))
	for _, v := range a.latestFrames {
		list = append(list, *v)
	}
	a.framesMu.RUnlock()
	return list
}

// Greet 데모용 메서드 (기존 유지)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// shutdown (선택) - 추후 Wails 종료 시 호출하도록 확장 가능
func (a *App) shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.conn != nil {
		_ = a.conn.Close()
	}
}
