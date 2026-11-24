program Datum;
uses Dos;
var
  J, M, T, W, h, min, s, hs: Word;
begin
  GetDate(J, M, T, W);
  GetTime(h, min, s, hs);
  WriteLn((J >= 2024) and (M >= 1) and (M <= 12) and (T >= 1) and (T <= 31) and (W <= 6));
  WriteLn((h <= 23) and (min <= 59) and (s <= 59) and (hs <= 99))
end.
