import type { UserInfo } from '@vben/types';

import { getStashedUserInfo } from './auth';

/**
 * gin 后端无独立 user/info 端点——登录时一次性返回 userInfo（LoginUserRes），
 * 已由 loginApi 缓存（localStorage，刷新可恢复）。此处复用缓存并映射到 vben UserInfo。
 *
 * LoginUserRes = {id, userName, employeeNo, name, userNickname, avatar,
 *   deptId, isAdmin, ...}
 */

export async function getUserInfoApi(): Promise<UserInfo> {
  const u = getStashedUserInfo() ?? {};
  return {
    userId: u.id ?? '',
    username: u.userName ?? '',
    realName: u.name ?? u.userNickname ?? '',
    avatar: u.avatar ?? '',
    // gin 权限走 accessCodes（permissions），roles 留空
    roles: [],
    // homePath: 留空则走 preferences.app.defaultHomePath
  } as unknown as UserInfo;
}
