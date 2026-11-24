program Farben;
uses Crt;
begin
  ClrScr;
  TextColor(Yellow);
  Write('gelb');
  TextBackground(Blue);
  TextColor(LightGreen + Blink);
  Write('blinkt');
  TextColor(Red + 16);
  HighVideo;
  LowVideo;
  NormVideo;
  WriteLn(' normal ', TextAttr);
  ClrEol; InsLine; DelLine; Sound(440); NoSound;
  WriteLn('Ende')
end.
