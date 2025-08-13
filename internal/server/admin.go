// admin.go: Admin 구독 처리 (Overview / Detail / Events)
// 관리자(Admin) 클라이언트의 프레임/이벤트 구독 스트림을 담당합니다.
// Overview: 전체 프레임 미리보기, Detail: 특정 Agent 프레임, Events: 특정 Agent 이벤트

package server

import (
	"log"
	"sync"

	"admin/proto"
)

const (
	// 채널 버퍼 크기 (CONTRIBUTING.md 기준)
	FRAME_CHANNEL_BUFFER_SIZE = 4096
	// 에이전트 오프라인 상태를 알리기 위한 특수 타임스탬프 값
	OFFLINE_TIMESTAMP = 0
)

// adminSubscriber는 Admin의 구독 정보를 저장합니다.
type adminSubscriber struct {
	adminId   string
	frameChan chan *proto.FrameData
	eventChan chan *proto.EventData
	closeOnce sync.Once
	closeFn   func()
}

// newAdminSubscriber는 adminSubscriber를 생성합니다.
func newAdminSubscriber(adminId string) *adminSubscriber {
	return &adminSubscriber{
		adminId:   adminId,
		frameChan: make(chan *proto.FrameData, FRAME_CHANNEL_BUFFER_SIZE),
		eventChan: make(chan *proto.EventData, FRAME_CHANNEL_BUFFER_SIZE),
	}
}

// newOfflineFrame는 에이전트 오프라인을 표현하는 FrameData를 생성합니다.
// 이미지 데이터는 비워두고, 타임스탬프를 OFFLINE_TIMESTAMP(=0)으로 설정합니다.
func newOfflineFrame(agentId string) *proto.FrameData {
	return &proto.FrameData{
		AgentId:   agentId,
		ImageData: []byte{}, // 빈 데이터로 오프라인 신호
		Timestamp: OFFLINE_TIMESTAMP,
		IsPreview: true, // Overview 스트림에서도 식별 가능하도록 preview 표시 유지
	}
}

// isOfflineFrame는 주어진 프레임이 오프라인 신호인지 판단합니다.
func isOfflineFrame(frame *proto.FrameData) bool {
	if frame == nil {
		return false
	}
	return frame.Timestamp == OFFLINE_TIMESTAMP && len(frame.ImageData) == 0
}

// close 안전하게 구독 채널을 닫습니다.
func (a *adminSubscriber) close() {
	a.closeOnce.Do(func() {
		close(a.frameChan)
		close(a.eventChan)
		if a.closeFn != nil {
			a.closeFn()
		}
	})
}

// AdminService 구현체
// Overview/Detail/Events 구독 메서드만 포함

type AdminService struct {
	proto.UnimplementedAdminServiceServer
	// 구독자 관리용 Mutex 및 맵
	overviewSubs map[string]*adminSubscriber
	detailSubs   map[string]map[string]*adminSubscriber // adminId -> agentId -> sub
	eventSubs    map[string]map[string]*adminSubscriber
	mu           sync.RWMutex
}

// NewAdminService는 AdminService를 생성합니다.
func NewAdminService() *AdminService {
	return &AdminService{
		overviewSubs: make(map[string]*adminSubscriber),
		detailSubs:   make(map[string]map[string]*adminSubscriber),
		eventSubs:    make(map[string]map[string]*adminSubscriber),
	}
}

// SubscribeOverview는 전체 프레임 미리보기를 스트리밍합니다.
func (s *AdminService) SubscribeOverview(req *proto.AdminSubscribeRequest, stream proto.AdminService_SubscribeOverviewServer) error {
	adminId := req.GetAdminId()
	sub := newAdminSubscriber(adminId)

	s.mu.Lock()
	s.overviewSubs[adminId] = sub
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.overviewSubs, adminId)
		s.mu.Unlock()
		sub.close()
		log.Printf("[Admin][%s] overview 구독 종료", adminId)
	}()

	log.Printf("[Admin][%s] overview 구독 시작", adminId)
	for frame := range sub.frameChan {
		if err := stream.Send(frame); err != nil {
			log.Printf("[Admin][%s] overview 전송 오류: %v", adminId, err)
			return err
		}
	}
	return nil
}

// SubscribeDetail는 특정 Agent의 프레임을 스트리밍합니다.
func (s *AdminService) SubscribeDetail(req *proto.AgentDetailRequest, stream proto.AdminService_SubscribeDetailServer) error {
	adminId := req.GetAdminId()
	agentId := req.GetAgentId()
	sub := newAdminSubscriber(adminId)

	s.mu.Lock()
	if s.detailSubs[adminId] == nil {
		s.detailSubs[adminId] = make(map[string]*adminSubscriber)
	}
	s.detailSubs[adminId][agentId] = sub
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.detailSubs[adminId], agentId)
		if len(s.detailSubs[adminId]) == 0 {
			delete(s.detailSubs, adminId)
		}
		s.mu.Unlock()
		sub.close()
		log.Printf("[Admin][%s] detail(%s) 구독 종료", adminId, agentId)
	}()

	log.Printf("[Admin][%s] detail(%s) 구독 시작", adminId, agentId)
	for frame := range sub.frameChan {
		if err := stream.Send(frame); err != nil {
			log.Printf("[Admin][%s] detail(%s) 전송 오류: %v", adminId, agentId, err)
			return err
		}
	}
	return nil
}

// SubscribeEvents는 특정 Agent의 이벤트를 스트리밍합니다.
func (s *AdminService) SubscribeEvents(req *proto.AgentDetailRequest, stream proto.AdminService_SubscribeEventsServer) error {
	adminId := req.GetAdminId()
	agentId := req.GetAgentId()
	sub := newAdminSubscriber(adminId)

	s.mu.Lock()
	if s.eventSubs[adminId] == nil {
		s.eventSubs[adminId] = make(map[string]*adminSubscriber)
	}
	s.eventSubs[adminId][agentId] = sub
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.eventSubs[adminId], agentId)
		if len(s.eventSubs[adminId]) == 0 {
			delete(s.eventSubs, adminId)
		}
		s.mu.Unlock()
		sub.close()
		log.Printf("[Admin][%s] events(%s) 구독 종료", adminId, agentId)
	}()

	log.Printf("[Admin][%s] events(%s) 구독 시작", adminId, agentId)
	for event := range sub.eventChan {
		if err := stream.Send(event); err != nil {
			log.Printf("[Admin][%s] events(%s) 전송 오류: %v", adminId, agentId, err)
			return err
		}
	}
	return nil
}

// broadcastOverview는 overview 구독자에게 프레임을 전달합니다.
func (s *AdminService) broadcastOverview(frame *proto.FrameData) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sub := range s.overviewSubs {
		select {
		case sub.frameChan <- frame:
		default:
			log.Printf("[Admin][%s] overview 채널 full", sub.adminId)
		}
	}
}

// broadcastDetail는 detail 구독자에게 프레임을 전달합니다.
func (s *AdminService) broadcastDetail(agentId string, frame *proto.FrameData) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sub := range s.detailSubs {
		if s, ok := sub[agentId]; ok {
			select {
			case s.frameChan <- frame:
			default:
				log.Printf("[Admin][%s] detail(%s) 채널 full", s.adminId, agentId)
			}
		}
	}
}

// broadcastEvents는 events 구독자에게 이벤트를 전달합니다.
func (s *AdminService) broadcastEvents(agentId string, event *proto.EventData) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sub := range s.eventSubs {
		if s, ok := sub[agentId]; ok {
			select {
			case s.eventChan <- event:
			default:
				log.Printf("[Admin][%s] events(%s) 채널 full", s.adminId, agentId)
			}
		}
	}
}

// PublishAgentOffline는 외부(Agent 연결 관리 로직)에서 호출하여
// 해당 에이전트가 오프라인 되었음을 모든 관련 구독자에게 알립니다.
// Overview 및 Detail 구독자에게 오프라인 프레임을 전송합니다.
func (s *AdminService) PublishAgentOffline(agentId string) {
	offlineFrame := newOfflineFrame(agentId)
	// Overview 전체 프레임 스트림으로 전송
	s.broadcastOverview(offlineFrame)
	// Detail 구독자(해당 agentId)를 대상으로 전송
	s.broadcastDetail(agentId, offlineFrame)
	log.Printf("[Agent][%s] offline 프레임 전송 완료", agentId)
}

// HandleIncomingFrame는 외부에서 들어온 프레임을 Admin 구독자에게 배포하는 헬퍼입니다.
// 오프라인 프레임 여부를 판단하고 그대로 전달합니다.
// NOTE: 필요 시 추가적인 필터링/캐싱 로직을 여기서 확장할 수 있습니다.
func (s *AdminService) HandleIncomingFrame(frame *proto.FrameData) {
	if frame == nil {
		return
	}
	// Overview 전송 (preview 여부는 클라이언트 로직에 따라 판단)
	s.broadcastOverview(frame)
	// Detail (특정 agent) 전송
	s.broadcastDetail(frame.AgentId, frame)
	if isOfflineFrame(frame) {
		log.Printf("[Agent][%s] offline 프레임 처리", frame.AgentId)
	}
}
