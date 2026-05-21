import { type Ref, inject, ref } from 'vue'

export const FORM_ITEM_ID_KEY = 'formItemId'

export function useFormItemId() {
  return inject<Ref<string>>(FORM_ITEM_ID_KEY, ref(''))
}
