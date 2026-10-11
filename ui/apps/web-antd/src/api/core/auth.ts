import type { Recordable } from '@vben/types';

import { baseRequestClient, requestClient } from '#/api/request';

/**
 * gin 后端登录契约：
 * - POST /api/v1/system/login  {username,password,verifyCode,verifyKey}
 *   → {code:0, data:{token,userInfo,menuList,permissions}}
 * - gin 登录一次性返回 userInfo+permissions，无独立 user/info、auth/codes 端点；
 *   故登录后把 userInfo/permissions 缓存（localStorage，刷新可恢复），
 *   供 getUserInfoApi / getAccessCodesApi 复用。
 * - captcha: GET /api/v1/pub/captcha/get → {code:0, data:{key,img}}
 */

export namespace AuthApi {
  /** 登录接口参数（verifyCode 后端 binding required） */
  export interface LoginParams {
    username: string;
    password: string;
    verifyCode: string;
    verifyKey?: string;
  }

  /** 登录接口返回值（vben 约定 {accessToken}） */
  export interface LoginResult {
    accessToken: string;
  }

  /** gin 登录 data 原始结构 */
  export interface GinLoginData {
    token: string;
    userInfo: any;
    menuList: any[];
    permissions: string[];
  }

  export interface CaptchaResult {
    key: string;
    img: string;
  }
}

const LS_KEY = 'aiarsenal.loginData';
let _userInfo: any = null;
let _accessCodes: string[] = [];

function stash(data: AuthApi.GinLoginData | undefined | null) {
  _userInfo = data?.userInfo ?? null;
  _accessCodes = data?.permissions ?? [];
  try {
    localStorage.setItem(
      LS_KEY,
      JSON.stringify({ userInfo: _userInfo, accessCodes: _accessCodes }),
    );
  } catch {
    /* 忽略 */
  }
}

function loadStash() {
  if (_userInfo || _accessCodes.length) return;
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) {
      const o = JSON.parse(raw);
      _userInfo = o.userInfo ?? null;
      _accessCodes = o.accessCodes ?? [];
    }
  } catch {
    /* 忽略 */
  }
}

export function getStashedUserInfo() {
  loadStash();
  return _userInfo;
}

export function getStashedAccessCodes(): string[] {
  loadStash();
  return _accessCodes;
}

/** 获取图形验证码 */
export async function getCaptchaApi() {
  return requestClient.get<AuthApi.CaptchaResult>(
    '/api/v1/pub/captcha/get',
  );
}

/**
 * 登录（store 传 Recordable<any>，此处放宽以匹配；实际字段见 LoginParams）
 */
export async function loginApi(data: Recordable<any>) {
  const res = await requestClient.post<AuthApi.GinLoginData>(
    '/api/v1/system/login',
    data,
  );
  stash(res);
  return { accessToken: res?.token ?? '' };
}

/**
 * 刷新 accessToken（gin 无 refresh 端点；preferences.app.enableRefreshToken
 * 默认 false 不会调用，此处仅留 stub 以满足 request.ts 的 import）
 */
export async function refreshTokenApi() {
  return { data: '', status: 0 };
}

/**
 * 退出登录（gin: GET /api/v1/system/logout，无状态，直接丢弃 token）
 */
export async function logoutApi() {
  try {
    await baseRequestClient.get('/api/v1/system/logout');
  } finally {
    _userInfo = null;
    _accessCodes = [];
    localStorage.removeItem(LS_KEY);
  }
}

/**
 * 获取用户权限码（gin 登录已返回 permissions，复用缓存）
 */
export async function getAccessCodesApi() {
  return getStashedAccessCodes();
}

/**
 * 桌面客户端登录中转：浏览器登录成功后，把 deviceCode 绑定到当前用户。
 * 后端把 (userId,userName,isAdmin) 写入 redis（TTL 180s），App 端再走 exchange 换 JWT。
 * 需带 Bearer token（已由 requestClient 拦截器注入）。
 */
export async function clientBindApi(deviceCode: string) {
  return requestClient.post('/api/v1/system/client/bind', { deviceCode });
}
