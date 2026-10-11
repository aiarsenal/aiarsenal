import type { VxeGridProps, VxeUIExport } from 'vxe-table';

import type { Recordable } from '@vben/types';

import type { VxeGridApi } from './api';

import { formatDate, formatDateTime, isFunction } from '@vben/utils';

export function extendProxyOptions(
  api: VxeGridApi,
  options: VxeGridProps,
  getFormValues: () => Recordable<any>,
) {
  [
    'query',
    'querySuccess',
    'queryError',
    'queryAll',
    'queryAllSuccess',
    'queryAllError',
  ].forEach((key) => {
    extendProxyOption(key, api, options, getFormValues);
  });
}

function extendProxyOption(
  key: string,
  api: VxeGridApi,
  options: VxeGridProps,
  getFormValues: () => Recordable<any>,
) {
  const { proxyConfig } = options;
  const configFn = (proxyConfig?.ajax as Recordable<any>)?.[key];
  if (!isFunction(configFn)) {
    return options;
  }

  const wrapperFn = async (
    params: Recordable<any>,
    customValues: Recordable<any>,
    ...args: Recordable<any>[]
  ) => {
    const formValues = getFormValues();
    /**
     * 开启toolbarConfig.refresh功能
     * 点击刷新按钮 这里的值为PointerEvent 会携带错误参数
     */
    const mergedFormValues = {
      ...(customValues instanceof PointerEvent ? {} : customValues),
      ...formValues,
    };
    // vben 用 vben-form 取代了 vxe 内置 form（adapter 中 formConfig.enabled=false），
    // 故 vxe 注入的 params.form 恒为空。这里把 vben-form 的提交值回填到 params.form，
    // 使各列表页 `query: async ({ page, form }) => ...` 的标准 vxe 写法能正确拿到搜索条件。
    // 同时仍作为第 2 个参数传入，兼容 `query: async (params, form) => ...` 写法。
    if (params && typeof params === 'object') {
      params.form = mergedFormValues;
    }
    const data = await configFn(params, mergedFormValues, ...args);
    return data;
  };
  api.setState({
    gridOptions: {
      proxyConfig: {
        ajax: {
          [key]: wrapperFn,
        },
      },
    },
  });
}

export function extendsDefaultFormatter(vxeUI: VxeUIExport) {
  vxeUI.formats.add('formatDate', {
    tableCellFormatMethod({ cellValue }) {
      return formatDate(cellValue);
    },
  });

  vxeUI.formats.add('formatDateTime', {
    tableCellFormatMethod({ cellValue }) {
      return formatDateTime(cellValue);
    },
  });
}
