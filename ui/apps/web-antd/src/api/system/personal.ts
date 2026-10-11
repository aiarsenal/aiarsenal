/**
 * gin 个人中心接口（/api/v1/system/personal/*）
 * 响应壳 {code:0, data:...}，requestClient 已返回 data 字段。
 */
import { requestClient } from '#/api/request';

export namespace PersonalApi {
  /** GET /personal/getPersonalInfo 返回 */
  export interface InfoResult {
    user: UserEntity;
    roles: string[];
    deptName: string;
  }

  /** PUT /personal/edit 请求体（gin DTO PersonalEditReq） */
  export interface EditReq {
    nickname: string;
    mobile: string;
    remark?: string;
    sex: number;
    userEmail: string;
    describe?: string;
    avatar?: string;
  }

  /** PUT /personal/edit 返回 */
  export interface EditRes {
    userInfo: UserEntity;
    token: string;
  }

  /** PUT /personal/resetPwd 请求体（gin DTO PersonalResetPwdReq） */
  export interface ResetPwdReq {
    password: string;
  }

  /** aa_user 实体（gin entity.User json 标签） */
  export interface UserEntity {
    id: string;
    userName?: string;
    employeeNo?: string;
    name?: string;
    mobile?: string;
    userNickname?: string;
    userEmail?: string;
    sex?: number;
    avatar?: string;
    deptId?: string;
    remark?: string;
    describe?: string;
    lastLoginIp?: string;
    lastLoginTime?: string;
    createdAt?: string;
    updatedAt?: string;
  }
}

/** 获取当前登录用户个人资料 */
export function getPersonalInfoApi() {
  return requestClient.get<PersonalApi.InfoResult>(
    '/api/v1/system/personal/getPersonalInfo',
  );
}

/** 编辑个人资料 */
export function editPersonalApi(data: PersonalApi.EditReq) {
  return requestClient.put<PersonalApi.EditRes>(
    '/api/v1/system/personal/edit',
    data,
  );
}

/** 修改个人密码 */
export function resetPersonalPwdApi(data: PersonalApi.ResetPwdReq) {
  return requestClient.put<null>('/api/v1/system/personal/resetPwd', data);
}
