/**
 * gin 岗位管理接口（/api/v1/system/post/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 *
 * gin DTO（dto/post.go）：
 *   PostSearchReq { postCode, postName, status, PageReq }
 *   PostSearchRes { PostList []*entity.Post (json:"postList"), ListRes{total} }
 *   PostReq       { postCode, postName, postSort, status(uint), remark }   // 新增
 *   PostEditReq   { postId(string), *PostReq }                             // 编辑
 *   PostDeleteReq { ids []string }                                         // 删除
 * entity.Post 行字段：postId, postCode, postName, postSort, status, remark,
 *                    createdBy, updatedBy, createdAt, updatedAt
 */
import type { Recordable } from '@vben/types';

import { requestClient } from '#/api/request';

export namespace PostApi {
  export interface ListParams {
    pageNum: number;
    pageSize: number;
    postCode?: string;
    postName?: string;
    status?: number | string;
  }

  export interface ListItem {
    postId: string;
    postCode: string;
    postName: string;
    postSort: number;
    status: number;
    remark?: string;
    createdBy?: string;
    updatedBy?: string;
    createdAt?: string;
    updatedAt?: string;
  }

  export interface ListResult {
    postList: ListItem[];
    total: number;
  }

  /** 新增 payload（PostReq） */
  export interface AddPayload {
    postCode: string;
    postName: string;
    postSort: number;
    status: number;
    remark?: string;
  }

  /** 编辑 payload（PostEditReq = postId + PostReq） */
  export interface EditPayload extends AddPayload {
    postId: string;
  }
}

/** 岗位列表 */
export function getPostListApi(params: PostApi.ListParams) {
  return requestClient.get<PostApi.ListResult>(
    '/api/v1/system/post/list',
    { params },
  );
}

/** 新增岗位 */
export function addPostApi(data: Recordable<any>) {
  return requestClient.post('/api/v1/system/post/add', data);
}

/** 编辑岗位 */
export function editPostApi(data: Recordable<any>) {
  return requestClient.put('/api/v1/system/post/edit', data);
}

/** 删除岗位（批量，body: { ids: [] }，ids 为业务 postId 字符串数组） */
export function deletePostApi(ids: string[]) {
  return requestClient.delete('/api/v1/system/post/delete', {
    data: { ids },
  });
}
