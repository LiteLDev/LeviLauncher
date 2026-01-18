import React from "react";
import leafLogo from "@/assets/images/ic_leaf_logo.png";
export const LeviIcon = ({ width = 48, height = 48, ...props }) => (
  <img
    src={leafLogo}
    width={width}
    height={height}
    alt="LeviLauncher logo"
    draggable="false"
    {...props}
  />
);
