/**
 * 测试环境准备。
 * @testing-library/vue 只在全局 afterEach 存在时才自动清理 DOM；
 * 本项目 vitest 不开 globals，所以显式注册，避免用例间残留上一轮的挂载结果。
 */
import { cleanup } from '@testing-library/vue'
import { afterEach } from 'vitest'

afterEach(() => {
  cleanup()
})
