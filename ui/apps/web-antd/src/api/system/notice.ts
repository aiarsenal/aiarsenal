/**
 * gin 通知接口（/api/v1/system/notice/*）
 * 响应壳 {code:0, data:...}，requestClient 返回 data。
 */
import { requestClient } from '#/api/request';

export namespace NoticeApi {
  export interface ListItem {
    noticeId: string;
    title: string;
    content: string;
    link?: string;
    type?: string; // info/warning/success/error
    isRead: number; // 0 未读 1 已读
    createdAt?: string;
  }

  export interface ListResult {
    noticeList: ListItem[];
    total: number;
  }

  export interface UnreadResult {
    unreadCount: number;
  }
}

/** 通知列表（支持按状态/类型/时间范围筛选 + 分页） */
export function getNoticeListApi(params: {
  pageNum: number;
  pageSize: number;
  isRead?: string; // "0"=未读 "1"=已读，空=全部
  type?: string; // info|warning|success|error，空=全部
  startTime?: string; // YYYY-MM-DD HH:MM:SS
  endTime?: string;
}) {
  return requestClient.get<NoticeApi.ListResult>(
    '/api/v1/system/notice/list',
    { params },
  );
}

/** 未读数 */
export function getNoticeUnreadCountApi() {
  return requestClient.get<NoticeApi.UnreadResult>(
    '/api/v1/system/notice/unreadCount',
  );
}

/** 标记单条已读 */
export function readNoticeApi(noticeId: string) {
  return requestClient.put('/api/v1/system/notice/read', { noticeId });
}

/** 全部已读 */
export function readAllNoticeApi() {
  return requestClient.put('/api/v1/system/notice/readAll');
}

/** 删除 */
export function deleteNoticeApi(noticeId: string) {
  return requestClient.delete('/api/v1/system/notice/delete', {
    data: { noticeId },
  });
}
