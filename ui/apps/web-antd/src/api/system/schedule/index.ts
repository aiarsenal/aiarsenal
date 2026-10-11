/**
 * 日程模块 — 我的日程/会议室接口（/api/v1/schedule/*）。
 * 时间串约定 "YYYY-MM-DD HH:mm[:ss]"（后端宽松解析，也接受纯日期）。
 * 响应壳 {code:0,data:...}，requestClient 已返回 data 字段。
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace ScheduleApi {
  /** 一场（周期展开后的实例） */
  export interface Occurrence {
    eventId: string;
    title: string;
    eventType: number; // 0日程 1线下会议 2线上会议 3线上+线下
    start: string;
    end: string;
    date: string;
    roomId: string;
    roomName: string;
    location: string;
    onlineProvider: string;
    onlineUrl: string;
    organizerId: string;
    organizerName: string;
    attendees: Attendee[];
    isRecurring: number;
    repeatDesc: string;
    modified: boolean;
    myStatus: number; // 0待确认 1接受 2拒绝；非参会人=-1
    isOrganizer: boolean;
    remark: string;
  }
  export interface Attendee {
    userId: string;
    userName: string;
    status: number;
  }
  export interface Exception {
    date: string;
    status: number; // 0取消 1改期
    newStartAt: string;
    newEndAt: string;
    newRoomId: string;
    newRoomName: string;
  }
  export interface Detail extends Occurrence {
    exceptions: Exception[];
  }
  export interface Conflict {
    eventId: string;
    title: string;
    roomId: string;
    roomName: string;
    start: string;
    end: string;
  }
  /** 创建/编辑返回的预览（Created=false + Conflicts=房间冲突待确认） */
  export interface Preview {
    eventId: string;
    title: string;
    eventType: number;
    startAt: string;
    endAt: string;
    repeatDesc: string;
    roomId: string;
    roomName: string;
    onlineUrl: string;
    location: string;
    attendees: Attendee[];
    conflicts: Conflict[];
    created: boolean;
  }
  export interface Room {
    roomId: string;
    name: string;
    location: string;
    capacity: number;
    equipment: string; // jsonb 文本 ["电视","白板"]
    enabled: number;
    remark: string;
    createdAt?: string;
  }
  export interface RoomDayBusy {
    eventId: string;
    title: string;
    start: string;
    end: string;
    organizerName: string;
  }
  /** 会议室当日时间轴行 */
  export interface RoomDay {
    roomId: string;
    name: string;
    location: string;
    capacity: number;
    equipment: string;
    enabled: number;
    remark: string;
    busy: RoomDayBusy[];
  }
  export interface EventInput {
    eventId?: string;
    title: string;
    eventType: number;
    startAt: string;
    endAt: string;
    rrule: string;
    roomId: string;
    onlineProvider?: string;
    onlineUrl: string;
    location: string;
    attendees: string[];
    remark: string;
    ignoreConflicts?: boolean;
    dryRun?: boolean;
  }
}

const BASE = '/api/v1/schedule';

// ===== 我的日程 =====
export function getMyScheduleApi(params: { from?: string; to?: string }) {
  return requestClient.get<{ list: ScheduleApi.Occurrence[] }>(
    `${BASE}/event/list`,
    { params },
  );
}
export function getScheduleDetailApi(eventId: string) {
  return requestClient.get<ScheduleApi.Detail>(`${BASE}/event/detail`, {
    params: { eventId },
  });
}
export function addScheduleEventApi(data: ScheduleApi.EventInput) {
  return requestClient.post<ScheduleApi.Preview>(`${BASE}/event/add`, data);
}
export function editScheduleEventApi(data: ScheduleApi.EventInput) {
  return requestClient.put<ScheduleApi.Preview>(`${BASE}/event/edit`, data);
}
export function deleteScheduleEventApi(ids: string[]) {
  return requestClient.delete(`${BASE}/event/delete`, { data: { ids } });
}
/** 单场取消/改期（周期事件专用；status 0=取消该场 1=改期） */
export function addScheduleExceptionApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/event/exception`, data);
}
/** 参会回执（status 1=接受 2=拒绝） */
export function respondScheduleApi(data: { eventId: string; status: number }) {
  return requestClient.post(`${BASE}/attendee/respond`, data);
}
export function getFreeBusyApi(params: Recordable<any>) {
  return requestClient.get(`${BASE}/freebusy`, { params });
}
export function findFreeSlotsApi(params: Recordable<any>) {
  return requestClient.get<{ list: any[] }>(`${BASE}/freeslots`, { params });
}

// ===== 会议室 =====
export function getRoomListApi(params: Recordable<any>) {
  return requestClient.get<{ list: ScheduleApi.Room[]; total: number }>(
    `${BASE}/room/list`,
    { params },
  );
}
export function getRoomOverviewApi(date: string) {
  return requestClient.get<{ list: ScheduleApi.RoomDay[]; date: string }>(
    `${BASE}/room/overview`,
    { params: { date } },
  );
}
export function addRoomApi(data: Recordable<any>) {
  return requestClient.post(`${BASE}/room/add`, data);
}
export function editRoomApi(data: Recordable<any>) {
  return requestClient.put(`${BASE}/room/edit`, data);
}
export function deleteRoomApi(ids: string[]) {
  return requestClient.delete(`${BASE}/room/delete`, { data: { ids } });
}
