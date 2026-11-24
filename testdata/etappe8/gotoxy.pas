program Positionen;
{ GotoXY ueber ANSI: ESC[y;xH. Spalte oder Zeile kleiner als 1 lassen den
  Cursor stehen; eine Obergrenze gibt es nicht. }
uses Crt;
begin
  ClrScr;
  GotoXY(10, 5);
  Write('A');
  GotoXY(0, 3);
  GotoXY(4, 0);
  Write('B');
  GotoXY(1, 1);
  Write('C');
  GotoXY(120, 40);
  WriteLn('D')
end.
