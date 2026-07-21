import { mount } from '@vue/test-utils'

import ResourceFormField from './ResourceFormField.vue'

const options = [
  { value: 1, label: '研发部 · ID 1', children: [{ value: 2, label: '研发一组 · ID 2' }] }
]

function mountField(type: 'select' | 'relation' | 'tree') {
  return mount(ResourceFormField, {
    props: {
      modelValue: type === 'select' ? 'email' : 1,
      field: {
        key: 'target',
        label: '目标',
        type,
        options: type === 'select' ? [{ value: 'email', label: '邮件' }] : undefined
      },
      options
    },
    global: {
      stubs: {
        ElSelect: {
          props: ['modelValue'],
          template: '<select data-testid="select"><slot /></select>'
        },
        ElOption: {
          props: ['label', 'value'],
          template: '<option :value="value">{{ label }}</option>'
        },
        ElTreeSelect: {
          props: ['modelValue', 'data'],
          template: '<div data-testid="tree-select" :data-count="data.length" />'
        }
      }
    }
  })
}

describe('通用资源表单字段', () => {
  it('枚举和关联字段渲染为下拉选择', () => {
    expect(mountField('select').get('[data-testid="select"]').text()).toContain('邮件')
    expect(mountField('relation').get('[data-testid="select"]').text()).toContain('研发部')
  })

  it('层级关联字段渲染为树选择', () => {
    expect(mountField('tree').get('[data-testid="tree-select"]').attributes('data-count')).toBe('1')
  })
})
