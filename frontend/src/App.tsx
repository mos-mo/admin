import {useEffect, useState} from 'react'

// 상수 정의
// 오프라인 판별용 특수 타임스탬프
const OFFLINE_TIMESTAMP = 0
// 미리보기 타일 가로 픽셀 크기 (확대 요청 반영)
const PREVIEW_TILE_WIDTH = 360
// 미리보기 이미지 종횡비 (더 와이드하게 16:9)
const PREVIEW_ASPECT_RATIO = '16/9'
// 디테일(확대) 뷰 최대 가로 폭
const DETAIL_VIEW_MAX_WIDTH = 1280
// 디테일 뷰 이미지 종횡비 (필요 시 별도 적용 가능 - 현재 동일)
const DETAIL_ASPECT_RATIO = '16/9'

// 개별 프레임 데이터 타입
interface OverviewFrameData {
    agentId: string
    imageBase64: string
    isPreview: boolean
    timestamp: number
}

// Wails Events API (런타임 전역)
declare global {
    interface Window {
        runtime?: any
    }
}

// 간단한 시간 포맷터
const formatTime = (ts: number) => {
    const d = new Date(ts)
    return d.toLocaleTimeString()
}

function App() {
    const [frames, setFrames] = useState<Record<string, OverviewFrameData>>({})
    // 선택된 디테일 에이전트 ID (없으면 undefined)
    const [selectedAgentId, setSelectedAgentId] = useState<string | undefined>(undefined)

    useEffect(() => {
        // 이벤트 수신 핸들러 (오프라인 프레임 → 제거)
        const handler = (data: OverviewFrameData) => {
            setFrames(prev => {
                const isOffline = data.timestamp === OFFLINE_TIMESTAMP && !data.imageBase64
                if (isOffline) {
                    const next = {...prev}
                    delete next[data.agentId]
                    return next
                }
                return {...prev, [data.agentId]: data}
            })
        }
        // Wails v2 이벤트 구독
        window.runtime?.EventsOn?.('overviewFrame', handler)
        return () => {
            window.runtime?.EventsOff?.('overviewFrame')
        }
    }, [])

    const frameList = Object.values(frames).sort((a, b) => b.timestamp - a.timestamp)
    const selectedFrame = selectedAgentId ? frames[selectedAgentId] : undefined

    // 디테일 선택 핸들러
    const handleSelectDetail = (agentId: string) => {
        setSelectedAgentId(agentId)
    }
    // 디테일 닫기 핸들러
    const handleCloseDetail = () => {
        setSelectedAgentId(undefined)
    }

    // 디테일 뷰 렌더 함수 (단일 책임 분리)
    const renderDetailView = () => {
        if (!selectedFrame) return null
        return (
            <div style={{padding: 16}}>
                <div style={{display: 'flex', alignItems: 'center', marginBottom: 12, gap: 12}}>
                    <button
                        onClick={handleCloseDetail}
                        style={{
                            background: '#222',
                            color: '#eee',
                            border: '1px solid #444',
                            borderRadius: 6,
                            padding: '6px 14px',
                            cursor: 'pointer'
                        }}
                    >
                        ← 목록으로
                    </button>
                    <h3 style={{margin: 0}}>{selectedFrame.agentId} 상세 화면</h3>
                    <span style={{fontSize: 12, color: '#0af'}}>LIVE</span>
                </div>
                <div
                    style={{
                        maxWidth: DETAIL_VIEW_MAX_WIDTH,
                        margin: '0 auto',
                        background: '#000',
                        border: '1px solid #333',
                        borderRadius: 12,
                        padding: 12
                    }}
                >
                    {selectedFrame.imageBase64 ? (
                        <img
                            src={`data:image/jpeg;base64,${selectedFrame.imageBase64}`}
                            style={{
                                width: '100%',
                                aspectRatio: DETAIL_ASPECT_RATIO,
                                objectFit: 'contain',
                                background: '#111',
                                borderRadius: 8
                            }}
                            alt={selectedFrame.agentId}
                        />
                    ) : (
                        <div
                            style={{
                                width: '100%',
                                aspectRatio: DETAIL_ASPECT_RATIO,
                                background: '#111',
                                borderRadius: 8,
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center',
                                color: '#555'
                            }}
                        >
                            영상 없음
                        </div>
                    )}
                    <div style={{marginTop: 8, fontSize: 12, color: '#bbb'}}>
                        Timestamp: {formatTime(selectedFrame.timestamp)}
                    </div>
                </div>
            </div>
        )
    }

    // 목록(Overview) 렌더 함수
    const renderOverviewGrid = () => (
        <div
            style={{
                display: 'grid',
                gridTemplateColumns: `repeat(auto-fill, ${PREVIEW_TILE_WIDTH}px)`,
                gap: '16px',
                alignItems: 'start'
            }}
        >
            {frameList.map(f => (
                <div
                    key={f.agentId}
                    style={{
                        border: '1px solid #444',
                        borderRadius: 12,
                        padding: 10,
                        background: '#101010',
                        boxShadow: '0 2px 6px rgba(0,0,0,0.4)'
                    }}
                >
                    <div style={{fontSize: 12, marginBottom: 6, display: 'flex', justifyContent: 'space-between'}}>
                        <div>
                            <strong>{f.agentId}</strong>
                            <span style={{marginLeft: 4, color: f.isPreview ? '#0af' : '#fa0'}}>
                                {/* {f.isPreview ? '목록보기중' : '상세보기중} */}
                            </span>
                        </div>
                        <button
                            onClick={() => handleSelectDetail(f.agentId)}
                            style={{
                                background: '#222',
                                color: '#ddd',
                                border: '1px solid #444',
                                borderRadius: 4,
                                padding: '2px 8px',
                                fontSize: 11,
                                cursor: 'pointer'
                            }}
                        >
                            디테일
                        </button>
                    </div>
                    {f.imageBase64 ? (
                        <img
                            src={`data:image/jpeg;base64,${f.imageBase64}`}
                            style={{
                                width: '100%',
                                aspectRatio: PREVIEW_ASPECT_RATIO,
                                objectFit: 'cover',
                                background: '#111',
                                borderRadius: 8,
                                border: '1px solid #222'
                            }}
                            alt={f.agentId}
                        />
                    ) : (
                        <div
                            style={{
                                width: '100%',
                                aspectRatio: PREVIEW_ASPECT_RATIO,
                                background: '#222',
                                borderRadius: 8,
                                border: '1px solid #222'
                            }}
                        />
                    )}
                    <div style={{fontSize: 11, marginTop: 4, color: '#ccc'}}>{formatTime(f.timestamp)}</div>
                </div>
            ))}
        </div>
    )

    return (
        <div style={{padding: '12px', fontFamily: 'sans-serif'}}>
            <h2>모아소프트 연구소 감시 프로그램 _v2 _jhkim</h2>
            {selectedFrame ? renderDetailView() : renderOverviewGrid()}
        </div>
    )
}

export default App
