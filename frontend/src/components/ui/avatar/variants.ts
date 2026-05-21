import { type VariantProps, cva } from 'class-variance-authority'

export const avatarVariant = cva(
  'inline-flex items-center justify-center font-medium text-foreground rounded-full select-none shrink-0 bg-primary-soft text-primary overflow-hidden',
  {
    variants: {
      size: {
        sm: 'h-8 w-8 text-xs',
        base: 'h-10 w-10 text-sm',
        lg: 'h-14 w-14 text-base',
      },
      shape: {
        circle: 'rounded-full',
        square: 'rounded-md',
      },
    },
    defaultVariants: {
      size: 'base',
      shape: 'circle',
    },
  },
)

export type AvatarVariants = VariantProps<typeof avatarVariant>
