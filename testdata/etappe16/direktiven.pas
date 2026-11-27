{ VER50 statt VER40, Schalter A aus TP 5.0 }
program Direktiven;
begin
  {$IFDEF VER50} WriteLn('VER50'); {$ENDIF}
  {$IFDEF VER40} WriteLn('VER40'); {$ENDIF}
  {$IFOPT A+} WriteLn('A+'); {$ENDIF}
  {$A-}
  {$IFOPT A-} WriteLn('A-'); {$ENDIF}
  {$A+}
  {$IFOPT A+} WriteLn('wieder A+'); {$ENDIF}
end.
