import { Button, type ButtonProps } from "@heroui/react";
export type ButtonWithBorderGradientProps = ButtonProps & {
  background?: string;
};
export const ButtonWithBorderGradient = ({
  children,
  background = "--background",
  style,
  ...props
}: ButtonWithBorderGradientProps) => (
  <Button
    {...props}
    style={{
      border: "solid 2px transparent",
      backgroundImage: `linear-gradient(var(${background}), var(${background})), linear-gradient(to right, #F871A0, #9353D3)`,
      backgroundOrigin: "border-box",
      backgroundClip: "padding-box, border-box",
      ...style,
    }}
  >
    {children}
  </Button>
);
