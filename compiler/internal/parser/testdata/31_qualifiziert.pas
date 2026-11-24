program Qualifiziert;
uses Crt, Dos;
type
  TFarbe = Crt.Byte;
var
  Std, Min, Sek, Hs: Word;
  WriteLn: Integer;
begin
  System.WriteLn('qualifiziert');
  Crt.TextColor(Crt.Red);
  Dos.GetTime(Std, Min, Sek, Hs);
  WriteLn := 3;
  System.WriteLn(WriteLn, ' ', Std:2, ':', Min:2)
end.
